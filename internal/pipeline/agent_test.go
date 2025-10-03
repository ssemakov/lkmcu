package pipeline

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/ssemakov/lkmcu/internal/config"
	"github.com/ssemakov/lkmcu/internal/layout"
	lk "github.com/ssemakov/lkmcu/internal/livekit"
	"github.com/ssemakov/lkmcu/internal/logging"
)

func TestNewAgent(t *testing.T) {
	cfg := AgentConfig{
		Layout: layout.Default(),
		Logger: logging.NewNullLogger("test", "info"),
		Config: config.Config{
			Room:       "test-room",
			Identity:   "test-identity",
			LivekitURL: "ws://localhost:7880",
			APIKey:     "test-key",
			APISecret:  "test-secret",
		},
	}

	agent := NewAgent(cfg)

	if agent == nil {
		t.Fatal("NewAgent returned nil")
	}
	if agent.cfg.Config.Room != "test-room" {
		t.Errorf("want Room=test-room, got %s", agent.cfg.Config.Room)
	}
	if agent.cfg.Config.Identity != "test-identity" {
		t.Errorf("want Identity=test-identity, got %s", agent.cfg.Config.Identity)
	}

	if agent.roomSnap == nil {
		t.Error("expected non-nil room snapshot")
	}
	if agent.layoutView == nil {
		t.Error("expected non-nil layout view")
	}
	if agent.evCh != nil {
		t.Error("expected nil event channel before Run()")
	}
}

func TestAgent_Run_ContextCancellation(t *testing.T) {
	cfg := AgentConfig{
		Layout: layout.Default(),
		Logger: logging.NewNullLogger("test", "info"),
		Config: config.Config{
			Room:       "test-room",
			Identity:   "test-identity",
			LivekitURL: "ws://localhost:7880",
			APIKey:     "test-key",
			APISecret:  "test-secret",
		},
	}

	agent := NewAgent(cfg)
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel context immediately to test cancellation behavior
	cancel()

	err := agent.Run(ctx)
	if err != context.Canceled {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
}

func TestAgent_Run_ContextTimeout(t *testing.T) {
	cfg := AgentConfig{
		Layout: layout.Default(),
		Logger: logging.NewNullLogger("test", "info"),
		Config: config.Config{
			Room:       "test-room",
			Identity:   "test-identity",
			LivekitURL: "ws://localhost:7880",
			APIKey:     "test-key",
			APISecret:  "test-secret",
		},
	}

	agent := NewAgent(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := agent.Run(ctx)
	duration := time.Since(start)

	if err != context.DeadlineExceeded {
		t.Errorf("expected context.DeadlineExceeded error, got %v", err)
	}

	// Should take approximately 100ms + 50ms sleep
	if duration < 100*time.Millisecond || duration > 200*time.Millisecond {
		t.Errorf("expected duration around 150ms, got %v", duration)
	}
}

// Mock lk.Client for testing
type mockLkClient struct {
	events chan lk.Event
	subs   []*lk.Subscription
}

func newMockLkClient() *mockLkClient {
	return &mockLkClient{
		events: make(chan lk.Event, 10),
		subs:   []*lk.Subscription{},
	}
}

func (m *mockLkClient) JoinRoom(roomName, identity string) error { return nil }
func (m *mockLkClient) Close() error                             { return nil }
func (m *mockLkClient) Events() <-chan lk.Event                  { return m.events }
func (m *mockLkClient) SubscriptionSnapshot() []*lk.Subscription { return m.subs }

func TestAgent_applyEvent(t *testing.T) {
	agent := NewAgent(AgentConfig{
		Logger: logging.NewNullLogger("test", "info"),
	})

	event := RoomChangedEvent{
		LkEvents: []lk.Event{
			lk.ParticipantJoined{PID: "participant-1", Identity: "p1"},
			lk.TrackPublished{PID: "participant-1", TID: "track-1", Kind: "video"},
		},
	}

	agent.applyEvent(event)

	if _, ok := agent.roomSnap.Participants["participant-1"]; !ok {
		t.Fatal("participant not found in room snapshot after applyEvent")
	}

	p := agent.roomSnap.Participants["participant-1"]
	if _, ok := p.PublishedTracks["track-1"]; !ok {
		t.Fatal("track not found for participant after applyEvent")
	}

	event = RoomChangedEvent{
		LkEvents: []lk.Event{
			lk.ParticipantLeft{PID: "participant-1"},
		},
	}

	agent.applyEvent(event)

	if _, ok := agent.roomSnap.Participants["participant-1"]; ok {
		t.Fatal("participant should be deleted after ParticipantLeft event")
	}
	if _, ok := agent.roomSnap.Tracks["track-1"]; ok {
		t.Fatal("track should be deleted after ParticipantLeft event")
	}
}

func TestAgent_reconcileState(t *testing.T) {
	cases := []struct {
		name        string
		initialSnap *RoomSnapshot
		initialSubs []*lk.Subscription
		policy      *Policy
		layout      *layout.LayoutView
		expectedOps []Operations
	}{
		{
			name: "subscribe to new video track",
			initialSnap: func() *RoomSnapshot {
				s := NewRoomSnapshot()
				s.UpsertParticipant("p1", "p1_identity")
				s.UpsertTrack("p1", "t1", TrackVideo)
				return s
			}(),
			initialSubs: []*lk.Subscription{},
			policy:      &Policy{IncludeKinds: []TrackKind{TrackVideo, TrackAudio}},
			layout:      func() *layout.LayoutView { v := layout.NewLayoutView(); v.StageTracks["t1"] = "p1"; return v }(),
			expectedOps: []Operations{&SubscribeOp{TrackID: "t1", Action: VideoFull, Reason: "video on stage"}},
		},
		{
			name: "do nothing for existing subscription",
			initialSnap: func() *RoomSnapshot {
				s := NewRoomSnapshot()
				s.UpsertParticipant("p1", "p1_identity")
				s.UpsertTrack("p1", "t1", TrackVideo)
				return s
			}(),
			initialSubs: []*lk.Subscription{{TID: "t1", PID: "p1"}},
			policy:      &Policy{IncludeKinds: []TrackKind{TrackVideo, TrackAudio}},
			layout:      func() *layout.LayoutView { v := layout.NewLayoutView(); v.StageTracks["t1"] = "p1"; return v }(),
			expectedOps: []Operations{},
		},
		{
			name: "ignore track when policy excludes its kind",
			initialSnap: func() *RoomSnapshot {
				s := NewRoomSnapshot()
				s.UpsertParticipant("p1", "p1_identity")
				s.UpsertTrack("p1", "t1", TrackAudio)
				return s
			}(),
			initialSubs: []*lk.Subscription{},
			policy:      &Policy{ExcludeKinds: []TrackKind{TrackAudio}},
			layout:      func() *layout.LayoutView { v := layout.NewLayoutView(); v.StageTracks["t1"] = "p1"; return v }(),
			expectedOps: []Operations{},
		},
		{
			name: "unsubscribe from track of left participant",
			initialSnap: func() *RoomSnapshot {
				s := NewRoomSnapshot()
				return s
			}(),
			initialSubs: []*lk.Subscription{{TID: "t1", PID: "p1"}},
			policy:      &Policy{IncludeKinds: []TrackKind{TrackVideo, TrackAudio}},
			layout:      func() *layout.LayoutView { v := layout.NewLayoutView(); return v }(),
			expectedOps: []Operations{&UnsubscribeOp{TrackID: "t1", Reason: "not in decisions / participant left"}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			agent := NewAgent(AgentConfig{
				Logger: logging.NewNullLogger("test", "info"),
			})
			mockClient := newMockLkClient()
			agent.lk = mockClient

			agent.roomSnap = tc.initialSnap
			agent.layoutView = tc.layout
			mockClient.subs = tc.initialSubs

			// TODO: update to compare actual ops sent to lkClient
			decs := Decide(tc.policy, tc.layout, tc.initialSnap)
			ops := Diff(tc.initialSubs, decs)

			if diff := cmp.Diff(tc.expectedOps, ops); diff != "" {
				t.Errorf("unexpected subscription ops (-want +got):\n%s", diff)
			}
		})
	}
}
