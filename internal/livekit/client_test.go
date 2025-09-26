package livekit

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	lksdk "github.com/livekit/server-sdk-go/v2"
	"github.com/pion/webrtc/v4"
)

func TestNewClient(t *testing.T) {
	url := "ws://localhost:7880"
	apiKey := "test-key"
	apiSecret := "test-secret"

	client := NewClient(url, apiKey, apiSecret, nil)

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
}

func TestClient_Close_WithoutRoom(t *testing.T) {
	client := NewClient("ws://localhost:7880", "key", "secret", nil)

	err := client.Close()
	if err == nil {
		t.Error("expected error when closing without a room, got nil")
	}
}

func TestClient_Close_WithRoom(t *testing.T) {
	client := NewClient("ws://localhost:7880", "key", "secret", nil)
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
    c := NewClient("ws://x", "k", "s", nil)
    fake := []*Subscription{
        {TID: "t1", PID: "p1", Kind: "video", Source: "camera"},
        {TID: "t2", PID: "p2", Kind: "audio", Source: "mic"},
    }
    c.room = &lksdk.Room{}
    c.listSubs = func(_ *lksdk.Room) []*Subscription { return fake }

    got := c.SubscriptionSnapshot()

    if len(got) != len(fake) { t.Fatalf("len mismatch") }

		// mutate returned slice; shouldn't affect a second call
    _ = append(got[:0], got[1:]...)
    got2 := c.SubscriptionSnapshot()
    if len(got2) != len(fake) { t.Fatalf("snapshot not independent") }
}


type fakeRP struct{ sid, identity string }

func (f fakeRP) SID() string      { return f.sid }
func (f fakeRP) Identity() string { return f.identity }

type fakePub struct {
	sid  string
	kind lksdk.TrackKind
}

func (f fakePub) SID() string           { return f.sid }
func (f fakePub) Kind() lksdk.TrackKind { return f.kind }

type fakeWebRTCTrack struct { Id string; }
func (f fakeWebRTCTrack) ID() string { return f.Id }
func (f fakeWebRTCTrack) Kind() webrtc.RTPCodecType { return webrtc.RTPCodecTypeVideo }

func newClient() *Client {
	return NewClient("ws://localhost:7880", "key", "secret", nil)
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
	track := fakeWebRTCTrack{Id: "track1"}
	pub := fakePub{sid: "track1", kind: lksdk.TrackKindVideo}
	rp := fakeRP{sid: "participant1", identity: "user1"}

	type testCase struct {
		name   string
		invoke func(c *Client)
		want   any
	}

	cases := []testCase{
		{
			name:   "participant joined",
			invoke: func(c *Client) { c.onParticipantJoined(rp) },
			want:   ParticipantJoined{PID: rp.sid, Identity: rp.identity},
		},
		{
			name:   "participant left",
			invoke: func(c *Client) { c.onParticipantLeft(rp) },
			want:   ParticipantLeft{PID: rp.sid, Identity: rp.identity},
		},
		{
			name:   "track published",
			invoke: func(c *Client) { c.onTrackPublished(pub, rp) },
			want:   TrackPublished{PID: rp.sid, Identity: rp.identity, TID: pub.sid, Kind: pub.kind.String()},
		},
		{
			name:   "track unpublished",
			invoke: func(c *Client) { c.onTrackUnpublished(pub, rp) },
			want:   TrackUnpublished{PID: rp.sid, Identity: rp.identity, TID: pub.sid},
		},
		{
			name:   "track subscribed",
			invoke: func(c *Client) { c.onTrackSubscribed(track, pub, rp) },
			want:   TrackSubscribed{PID: rp.sid, Identity: rp.identity, TID: pub.sid, Kind: pub.kind.String(), Track: track},
		},
		{
			name:   "track unsubscribed",
			invoke: func(c *Client) { c.onTrackUnsubscribed(track, pub, rp) },
			want:   TrackUnsubscribed{PID: rp.sid, Identity: rp.identity, TID: pub.sid, Kind: pub.kind.String(), Track: track},
		},
		{
			name:   "room reconnected",
			invoke: func(c *Client) { c.onRoomReconnected() },
			want:   RoomReconnected{},
		},
		{
			name:   "room disconnected",
			invoke: func(c *Client) { c.onRoomDisconnected(lksdk.DisconnectionReason("server_terminated")) },
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

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Fatalf("event mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestPush_DropOldest(t *testing.T) {
    c := NewClient("ws://x", "k", "s", &EventOptions{BufferSize: 2, DropPolicy: DropOldest})

    c.push(ParticipantJoined{PID: "1"})
    c.push(ParticipantJoined{PID: "2"})
    c.push(ParticipantJoined{PID: "3"}) // should drop "1"

    got := []Event{<-c.Events(), <-c.Events()}
    if got[0].(ParticipantJoined).PID != "2" || got[1].(ParticipantJoined).PID != "3" {
        t.Fatalf("unexpected: %#v", got)
    }
}

func TestPush_Block(t *testing.T) {
    c := NewClient("ws://x", "k", "s", &EventOptions{BufferSize: 1, DropPolicy: Block})
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
    c := NewClient("ws://x","k","s", &EventOptions{BufferSize: 2, DropPolicy: Block})

		c.onParticipantJoined(fakeRP{sid:"1", identity:"a"})
    c.onParticipantJoined(fakeRP{sid:"2", identity:"b"})

    _ = c.Close()

		// can read buffered events after close
    got1 := <-c.Events()
    got2 := <-c.Events()
    if got1.(ParticipantJoined).PID != "1" || got2.(ParticipantJoined).PID != "2" {
        t.Fatalf("unexpected events: %#v %#v", got1, got2)
    }
}

func TestClose_NoNewEventsAfter(t *testing.T) {
    c := NewClient("ws://x","k","s", &EventOptions{BufferSize: 1, DropPolicy: Block})
    _ = c.Close()

    c.onParticipantJoined(fakeRP{sid:"3", identity:"c"})

    select {
    case e := <-c.Events():
        t.Fatalf("unexpected event after Close: %#v", e)
    case <-time.After(50 * time.Millisecond):
        // good: nothing produced
    }
}
