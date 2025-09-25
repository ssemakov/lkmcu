package livekit

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
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

type fakeRP struct{ sid, identity string }

func (f fakeRP) SID() string      { return f.sid }
func (f fakeRP) Identity() string { return f.identity }

type fakePub struct {
	sid  string
	kind lksdk.TrackKind
}

func (f fakePub) SID() string           { return f.sid }
func (f fakePub) Kind() lksdk.TrackKind { return f.kind }

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
	track := &webrtc.TrackRemote{}
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

			if diff := cmp.Diff(tc.want, got, cmpopts.IgnoreUnexported(webrtc.TrackRemote{})); diff != "" {
				t.Fatalf("event mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
