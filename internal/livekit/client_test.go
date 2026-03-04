package livekit

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	lksdk "github.com/livekit/server-sdk-go/v2"
	"github.com/pion/interceptor"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
)

func TestNewClient(t *testing.T) {
	url := "ws://localhost:7880"
	apiKey := "test-key"
	apiSecret := "test-secret"

	client := NewClient(url, apiKey, apiSecret, nil).(*client) // type assert to access internal fields for testing

	if client == nil {
		t.Fatal("NewClient returned nil")
	}
	if client.url != url {
		t.Errorf("want url=%s, got %s", url, client.url)
	}
	if client.apiKey != apiKey {
		t.Errorf("want apiKey=%s, got %s", apiKey, client.apiKey)
	}
	if client.apiSecret != apiSecret {
		t.Errorf("want apiSecret=%s, got %s", apiSecret, client.apiSecret)
	}
	if client.room != nil {
		t.Error("want room=nil, got non-nil room")
	}

	if client.evCh == nil {
		t.Error("want evCh initialized, got nil")
	}
	if cap(client.evCh) != 64 {
		t.Errorf("want evCh capacity=64, got %d", cap(client.evCh))
	}
	if client.dropPolicy != DropOldest {
		t.Errorf("want dropPolicy=DropOldest, got %d", client.dropPolicy)
	}

	if client.listSubsFn == nil {
		t.Fatal("listSubsFn is nil")
	}

	if client.findSubTargetFn == nil {
		t.Fatal("findSubTargetFn is nil")
	}
}

func TestClient_Subscribe(t *testing.T) {
	cases := []struct {
		name      string
		setup     func(c *client)
		pid       string
		tid       string
		subscribe bool
		wantErr   string
	}{
		{
			name:    "room is nil",
			setup:   func(c *client) { c.room = nil },
			pid:     "p1",
			tid:     "t1",
			wantErr: "room is not connected",
		},
		{
			name: "SetSubscribed returns error",
			setup: func(c *client) {
				c.room = &lksdk.Room{}
				c.findSubTargetFn = func(pid, tid string) (setSubscribedFn, error) {
					return func(sub bool) error { return errors.New("fail") }, nil
				}
			},
			pid:     "p1",
			tid:     "t1",
			wantErr: "fail",
		},
		{
			name: "success subscribe true",
			setup: func(c *client) {
				c.room = &lksdk.Room{}
				c.findSubTargetFn = func(pid, tid string) (setSubscribedFn, error) {
					return func(sub bool) error { return nil }, nil
				}
			},
			pid:       "p1",
			tid:       "t1",
			subscribe: true,
			wantErr:   "",
		},
		{
			name: "success subscribe false",
			setup: func(c *client) {
				c.room = &lksdk.Room{}
				c.findSubTargetFn = func(pid, tid string) (setSubscribedFn, error) {
					return func(sub bool) error { return nil }, nil
				}
			},
			pid:       "p1",
			tid:       "t1",
			subscribe: false,
			wantErr:   "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := NewClient("ws://x", "k", "s", nil).(*client)
			tc.setup(c)
			err := c.Subscribe(tc.pid, tc.tid, tc.subscribe)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			} else {
				if err == nil || err.Error() != tc.wantErr {
					t.Fatalf("want error %q, got %v", tc.wantErr, err)
				}
			}
		})
	}
}

func TestClient_Close_WithoutRoom(t *testing.T) {
	client := NewClient("ws://localhost:7880", "key", "secret", nil)

	err := client.Close()
	if err == nil {
		t.Error("expected error when closing without a room, got nil")
	}
}

func TestClient_Close_WithRoom(t *testing.T) {
	client := NewClient("ws://localhost:7880", "key", "secret", nil).(*client)
	client.room = lksdk.NewRoom(nil)

	err := client.Close()
	if err != nil {
		t.Fatalf("Close() returned error: %v", err)
	}
}
func TestSubscriptionSnapshot_NilRoomREturnsEmpty(t *testing.T) {
	c := NewClient("ws://x", "k", "s", nil)

	got := c.SubscriptionSnapshot()
	if len(got) != 0 {
		t.Errorf("got %d, want 0", len(got))
	}
}

func TestSubscriptionSnapshot_UsesListSubsSeam(t *testing.T) {
	c := NewClient("ws://x", "k", "s", nil).(*client) // type assert to access internal fields for testing
	fake := []*Subscription{
		{TID: "t1", PID: "p1", Kind: "video", Source: "camera"},
		{TID: "t2", PID: "p2", Kind: "audio", Source: "mic"},
	}
	c.room = &lksdk.Room{}
	c.listSubsFn = func() []*Subscription { return fake }

	got := c.SubscriptionSnapshot()

	if len(got) != len(fake) {
		t.Fatalf("len mismatch")
	}

	// mutate returned slice; shouldn't affect a second call
	_ = append(got[:0], got[1:]...)
	got2 := c.SubscriptionSnapshot()
	if len(got2) != len(fake) {
		t.Fatalf("snapshot not independent")
	}
}

type fakeRP struct{ sid, identity string }

func (f fakeRP) SID() string      { return f.sid }
func (f fakeRP) Identity() string { return f.identity }

type fakePub struct {
	sid  string
	kind lksdk.TrackKind
}

func (f fakePub) SID() string                   { return f.sid }
func (f fakePub) Kind() lksdk.TrackKind         { return f.kind }
func (f fakePub) Receiver() *webrtc.RTPReceiver { return nil }

type fakeRemoteTrack struct {
	IdVal   string
	KindVal string
	SsrcVal uint32
}

func (f fakeRemoteTrack) ID() string { return f.IdVal }

// func (f fakeRemoteTrack) Kind() string                    { return f.KindVal }
// func (f fakeRemoteTrack) ReadRTP() ([]byte, error)        { return nil, nil }
func (f fakeRemoteTrack) SSRC() webrtcSSRC                { return webrtc.SSRC(f.SsrcVal) }
func (f fakeRemoteTrack) SetReadDeadline(time.Time) error { return nil }
func (f fakeRemoteTrack) ReadRTP() (*rtp.Packet, interceptor.Attributes, error) {
	return nil, nil, nil
}

func newClient() *client {
	return NewClient("ws://localhost:7880", "key", "secret", nil).(*client)
}

func recvOne(t *testing.T, ch <-chan Event) any {
	t.Helper()
	select {
	case ev := <-ch:
		return ev
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for event")
		return nil
	}
}

func TestClient_EventHandling(t *testing.T) {
	track := fakeRemoteTrack{IdVal: "track1", KindVal: "video", SsrcVal: 1234}
	pub := fakePub{sid: "track1", kind: lksdk.TrackKindVideo}
	rp := fakeRP{sid: "participant1", identity: "user1"}

	type testCase struct {
		name   string
		invoke func(c *client)
		want   any
	}

	cases := []testCase{
		{
			name:   "participant joined",
			invoke: func(c *client) { c.onParticipantJoined(rp) },
			want:   ParticipantJoined{PID: rp.sid, Identity: rp.identity},
		},
		{
			name:   "participant left",
			invoke: func(c *client) { c.onParticipantLeft(rp) },
			want:   ParticipantLeft{PID: rp.sid, Identity: rp.identity},
		},
		{
			name:   "track published",
			invoke: func(c *client) { c.onTrackPublished(pub, rp) },
			want:   TrackPublished{PID: rp.sid, Identity: rp.identity, TID: pub.sid, Kind: pub.kind.String()},
		},
		{
			name:   "track unpublished",
			invoke: func(c *client) { c.onTrackUnpublished(pub, rp) },
			want:   TrackUnpublished{PID: rp.sid, Identity: rp.identity, TID: pub.sid},
		},
		{
			name:   "track subscribed",
			invoke: func(c *client) { c.onTrackSubscribed(track, pub, rp) },
			want: TrackSubscribed{PID: rp.sid, Identity: rp.identity, TID: pub.sid, Kind: pub.kind.String(),
				RemoteTrack: &remoteTrackAdapter{track: track}, Receiver: &receiverAdapter{receiver: nil}},
		},
		{
			name:   "track unsubscribed",
			invoke: func(c *client) { c.onTrackUnsubscribed(track, pub, rp) },
			want:   TrackUnsubscribed{PID: rp.sid, Identity: rp.identity, TID: pub.sid, Kind: pub.kind.String()},
		},
		{
			name:   "room reconnected",
			invoke: func(c *client) { c.onRoomReconnected() },
			want:   RoomReconnected{},
		},
		{
			name:   "room disconnected",
			invoke: func(c *client) { c.onRoomDisconnected(lksdk.DisconnectionReason("server_terminated")) },
			want:   RoomDisconnected{Reason: "server_terminated"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := newClient()
			tc.invoke(c)

			got := recvOne(t, c.Events())

			if gotT, wantT := fmt.Sprintf("%T", got), fmt.Sprintf("%T", tc.want); gotT != wantT {
				t.Fatalf("event type: want %s, got %s", wantT, gotT)
			}

			if diff := cmp.Diff(tc.want, got,
				cmpopts.IgnoreFields(TrackSubscribed{}, "RemoteTrack"),
				cmpopts.IgnoreFields(TrackUnsubscribed{}, "RemoteTrack"),
				cmpopts.IgnoreUnexported(receiverAdapter{}),
			); diff != "" {
				t.Fatalf("event mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestPush_DropOldest(t *testing.T) {
	c := NewClient("ws://x", "k", "s", &EventOptions{BufferSize: 2, DropPolicy: DropOldest}).(*client) // type assert to access internal fields for testing

	c.push(ParticipantJoined{PID: "1"})
	c.push(ParticipantJoined{PID: "2"})
	c.push(ParticipantJoined{PID: "3"}) // should drop "1"

	got := []Event{<-c.Events(), <-c.Events()}
	if got[0].(ParticipantJoined).PID != "2" || got[1].(ParticipantJoined).PID != "3" {
		t.Fatalf("unexpected: %#v", got)
	}
}

func TestPush_Block(t *testing.T) {
	c := NewClient("ws://x", "k", "s", &EventOptions{BufferSize: 1, DropPolicy: Block}).(*client) // type assert to access internal fields for testing
	c.push(ParticipantJoined{PID: "1"})

	blocked := make(chan struct{})
	go func() {
		c.push(ParticipantJoined{PID: "2"}) // should block until read
		close(blocked)
	}()

	select {
	case <-blocked:
		t.Fatal("push should block, but it returned early")
	case <-time.After(50 * time.Millisecond):
		// still blocked as expected
	}

	<-c.Events()
	select {
	case <-blocked: // now writer should have completed
	case <-time.After(200 * time.Millisecond):
		t.Fatal("push remained blocked after space available")
	}
}

func TestClose_BufferedEventsRemainReadable(t *testing.T) {
	c := NewClient("ws://x", "k", "s", &EventOptions{BufferSize: 2, DropPolicy: Block}).(*client) // type assert to access internal fields for testing

	c.onParticipantJoined(fakeRP{sid: "1", identity: "a"})
	c.onParticipantJoined(fakeRP{sid: "2", identity: "b"})

	_ = c.Close()

	// can read buffered events after close
	got1 := <-c.Events()
	got2 := <-c.Events()
	if got1.(ParticipantJoined).PID != "1" || got2.(ParticipantJoined).PID != "2" {
		t.Fatalf("unexpected events: %#v %#v", got1, got2)
	}
}

func TestClose_NoNewEventsAfter(t *testing.T) {
	c := NewClient("ws://x", "k", "s", &EventOptions{BufferSize: 1, DropPolicy: Block}).(*client) // type assert to access internal fields for testing
	_ = c.Close()

	c.onParticipantJoined(fakeRP{sid: "3", identity: "c"})

	select {
	case e := <-c.Events():
		t.Fatalf("unexpected event after Close: %#v", e)
	case <-time.After(50 * time.Millisecond):
		// good: nothing produced
	}
}
