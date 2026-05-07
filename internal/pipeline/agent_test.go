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
	"github.com/ssemakov/lkmcu/internal/media/packet"
	"github.com/ssemakov/lkmcu/internal/media/processor"
)

func TestNewAgent(t *testing.T) {
	cfg := AgentConfig{
		Logger: logging.NewNullLogger("test", "info"),
		Config: config.Config{
			Room:       "test-room",
			Identity:   "test-identity",
			LivekitURL: "ws://localhost:7880",
			APIKey:     "test-key",
			APISecret:  "test-secret",
		},
	}

	agent := NewAgent(cfg, layout.DefaultLayoutView())

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
	if agent.evCh == nil {
		t.Error("expected event channel to be set before Run()")
	}
}

func TestAgent_Run_ContextCancellation(t *testing.T) {
	cfg := AgentConfig{
		Logger: logging.NewNullLogger("test", "info"),
		Config: config.Config{
			Room:       "test-room",
			Identity:   "test-identity",
			LivekitURL: "ws://localhost:7880",
			APIKey:     "test-key",
			APISecret:  "test-secret",
		},
	}

	agent := NewAgent(cfg, layout.DefaultLayoutView())
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
		Logger: logging.NewNullLogger("test", "info"),
		Config: config.Config{
			Room:       "test-room",
			Identity:   "test-identity",
			LivekitURL: "ws://localhost:7880",
			APIKey:     "test-key",
			APISecret:  "test-secret",
		},
	}

	agent := NewAgent(cfg, layout.DefaultLayoutView())
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

func TestAgent_consumeLiveKitEvents_CancelsOnRoomClosed(t *testing.T) {
	agent := NewAgent(AgentConfig{
		Logger: logging.NewNullLogger("test", "info"),
	}, layout.DefaultLayoutView())

	mockClient := newMockLkClient()
	agent.lk = mockClient

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		agent.consumeLiveKitEvents(ctx, cancel)
		close(done)
	}()

	mockClient.events <- lk.RoomClosed{}

	select {
	case <-ctx.Done():
	case <-time.After(250 * time.Millisecond):
		t.Fatal("expected consumeLiveKitEvents to cancel context on RoomClosed")
	}

	select {
	case <-done:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("expected consumeLiveKitEvents goroutine to exit after RoomClosed")
	}
}

func TestAgent_TranslatesLiveKitEventsToRoomChangedEvents(t *testing.T) {
	agent := NewAgent(AgentConfig{
		Logger: logging.NewNullLogger("test", "info"),
	}, layout.DefaultLayoutView())
	mockLkClient := newMockLkClient()
	agent.lk = mockLkClient

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		agent.consumeLiveKitEvents(ctx, cancel)
		close(done)
	}()

	events := []lk.Event{
		lk.ParticipantJoined{PID: "p1", Identity: "alice"},
		lk.ParticipantLeft{PID: "p1", Identity: "alice"},
		lk.TrackPublished{PID: "p1", Identity: "alice", TID: "t1", Kind: "video"},
		lk.TrackUnpublished{PID: "p1", Identity: "alice", TID: "t1", Kind: "video"},
		lk.TrackSubscribed{PID: "p1", Identity: "alice", TID: "t1", Kind: "video"},
		lk.TrackUnsubscribed{PID: "p1", Identity: "alice", TID: "t1", Kind: "video"},
	}

	for _, event := range events {
		mockLkClient.events <- event
	}

	for _, want := range events {
		select {
		case got := <-agent.evCh:
			roomChanged, ok := got.(RoomChangedEvent)
			if !ok {
				t.Fatalf("expected RoomChangedEvent, got %T", got)
			}

			// RoomChangedEvent returns a slice []lk.Event
			if diff := cmp.Diff([]lk.Event{want}, roomChanged.LkEvents); diff != "" {
				t.Fatalf("unexpected LiveKit evnets (-want +got):\n%s", diff)
			}
		case <-time.After(500 * time.Millisecond):
			t.Fatal("expected RoomChangeEvent")
		}
	}

	cancel()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected consumeLiveKitEvents goroutine to exit after cancellation")
	}
}

func TestAgent_applyEvent_CancelsOnRoomDisconnected(t *testing.T) {
	agent := NewAgent(AgentConfig{
		Logger: logging.NewNullLogger("test", "info"),
	}, layout.DefaultLayoutView())
	mockLkClient := newMockLkClient()
	agent.lk = mockLkClient

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		agent.consumeLiveKitEvents(ctx, cancel)
		close(done)
	}()

	mockLkClient.events <- lk.RoomDisconnected{}

	select {
	case <-ctx.Done():
	case <-time.After(250 * time.Millisecond):
		t.Fatal("expected Run to cancel context on RoomDisconnected")
	}

	select {
	case <-done:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("expected Run goroutine to exit after RoomDisconnected")
	}
}

type fakeRemoteTrack struct {
	id   string
	kind string
	ssrc uint32
}

func (f fakeRemoteTrack) ID() string                      { return f.id }
func (f fakeRemoteTrack) Kind() string                    { return f.kind }
func (f fakeRemoteTrack) ReadRTP() (packet.Packet, error) { return packet.Packet{}, nil }
func (f fakeRemoteTrack) SSRC() uint32                    { return f.ssrc }
func (f fakeRemoteTrack) ClockRate() uint32               { return 90000 }
func (f fakeRemoteTrack) MimeType() string                { return "" }
func (f fakeRemoteTrack) SetReadDeadline(time.Time) error { return nil }

type noopTrackPipelineManager struct{}

func (noopTrackPipelineManager) NewTrackPipeline(lk.RemoteTrack, lk.Receiver) (processor.TrackPipeline, error) {
	return nil, nil
}

func (noopTrackPipelineManager) RemoveTrackPipeline(string) error {
	return nil
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
func (m *mockLkClient) Subscribe(pid, tid string, subscribe bool) error {
	if !subscribe {
		return m.unsubscribe(tid, pid)
	}

	m.subs = append(m.subs, &lk.Subscription{TID: tid, PID: pid})
	return nil
}

func (m *mockLkClient) unsubscribe(tid, pid string) error {
	for i, sub := range m.subs {
		if sub.TID == tid && sub.PID == pid {
			m.subs = append(m.subs[:i], m.subs[i+1:]...)
			return nil
		}
	}
	return nil
}

func TestAgent_applyEvent(t *testing.T) {
	type testCase struct {
		name   string
		setup  func(a *Agent)
		event  lk.Event
		assert func(t *testing.T, a *Agent)
	}

	cases := []testCase{
		{
			name:  "participant joined",
			event: lk.ParticipantJoined{PID: "p1", Identity: "alice"},
			assert: func(t *testing.T, a *Agent) {
				t.Helper()
				p, ok := a.roomSnap.Participants["p1"]
				if !ok {
					t.Fatal("participant not found after ParticipantJoined")
				}
				if p.Identity != "alice" {
					t.Fatalf("expected identity alice, got %s", p.Identity)
				}
			},
		},
		{
			name: "participant left",
			setup: func(a *Agent) {
				a.roomSnap.UpsertParticipant("p1", "alice")
				_, _ = a.roomSnap.UpsertTrack("p1", "t1", TrackVideo)
				a.roomSnap.UpsertSubCache("t1", VideoFull)
			},
			event: lk.ParticipantLeft{PID: "p1", Identity: "alice"},
			assert: func(t *testing.T, a *Agent) {
				t.Helper()
				if _, ok := a.roomSnap.Participants["p1"]; ok {
					t.Fatal("participant should be removed after ParticipantLeft")
				}
				if _, ok := a.roomSnap.Tracks["t1"]; ok {
					t.Fatal("track should be removed after ParticipantLeft")
				}
				if _, ok := a.roomSnap.SubCache["t1"]; ok {
					t.Fatal("subscription cache should be removed after ParticipantLeft")
				}
			},
		},
		{
			name: "track published",
			setup: func(a *Agent) {
				a.roomSnap.UpsertParticipant("p1", "alice")
			},
			event: lk.TrackPublished{PID: "p1", Identity: "alice", TID: "t1", Kind: "video"},
			assert: func(t *testing.T, a *Agent) {
				t.Helper()
				p := a.roomSnap.Participants["p1"]
				if p == nil {
					t.Fatal("participant not found after TrackPublished")
				}
				if _, ok := p.PublishedTracks["t1"]; !ok {
					t.Fatal("track not found in participant after TrackPublished")
				}
				if _, ok := a.roomSnap.Tracks["t1"]; !ok {
					t.Fatal("track not found in room snapshot after TrackPublished")
				}
			},
		},
		{
			name: "track unpublished",
			setup: func(a *Agent) {
				a.roomSnap.UpsertParticipant("p1", "alice")
				_, _ = a.roomSnap.UpsertTrack("p1", "t1", TrackVideo)
				a.roomSnap.UpsertSubCache("t1", VideoFull)
			},
			event: lk.TrackUnpublished{PID: "p1", Identity: "alice", TID: "t1", Kind: "video"},
			assert: func(t *testing.T, a *Agent) {
				t.Helper()
				if _, ok := a.roomSnap.Tracks["t1"]; ok {
					t.Fatal("track should be removed after TrackUnpublished")
				}
				if _, ok := a.roomSnap.SubCache["t1"]; ok {
					t.Fatal("subscription cache should be removed after TrackUnpublished")
				}
			},
		},
		{
			name: "track subscribed",
			setup: func(a *Agent) {
				a.roomSnap.UpsertParticipant("p1", "alice")
				_, _ = a.roomSnap.UpsertTrack("p1", "t1", TrackVideo)
			},
			event: lk.TrackSubscribed{
				PID:         "p1",
				Identity:    "alice",
				TID:         "t1",
				Kind:        "video",
				RemoteTrack: fakeRemoteTrack{id: "t1", kind: "video", ssrc: 1234},
			},
			assert: func(t *testing.T, a *Agent) {
				t.Helper()
				if got, ok := a.roomSnap.SubCache["t1"]; !ok || got != DecisionAction("video") {
					t.Fatalf("expected current subscription for t1 to be video, got %q (present=%v)", got, ok)
				}
			},
		},
		{
			name: "track unsubscribed",
			setup: func(a *Agent) {
				a.roomSnap.UpsertParticipant("p1", "alice")
				_, _ = a.roomSnap.UpsertTrack("p1", "t1", TrackVideo)
				a.roomSnap.UpsertSubCache("t1", VideoFull)
			},
			event: lk.TrackUnsubscribed{PID: "p1", Identity: "alice", TID: "t1", Kind: "video"},
			assert: func(t *testing.T, a *Agent) {
				t.Helper()
				if _, ok := a.roomSnap.Tracks["t1"]; ok {
					t.Fatal("track should be removed after TrackUnsubscribed")
				}
				if _, ok := a.roomSnap.SubCache["t1"]; ok {
					t.Fatal("subscription cache should be removed after TrackUnsubscribed")
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			agent := NewAgent(AgentConfig{
				Logger: logging.NewNullLogger("test", "info"),
			}, layout.DefaultLayoutView())
			agent.trackPipelineManager = noopTrackPipelineManager{}

			if tc.setup != nil {
				tc.setup(agent)
			}

			agent.applyEvent(RoomChangedEvent{LkEvents: []lk.Event{tc.event}})
			tc.assert(t, agent)
		})
	}
}

func TestAgent_applyEvent_LayoutChanged(t *testing.T) {
	agent := NewAgent(AgentConfig{
		Logger: logging.NewNullLogger("test", "info"),
	}, layout.DefaultLayoutView())

	stage := &layout.Stage{Slots: []layout.Slot{{Identity: "alice", TrackKind: "video"}}}

	agent.applyEvent(LayoutChangedEvent{Stage: stage})

	snap := agent.layoutView.Snapshot()

	if snap.CurrentStage != stage {
		t.Fatal("expected CurrentStage pointer to match stage from event")
	}

	kinds, ok := snap.StageParticipants["alice"]
	if !ok {
		t.Fatalf("expected alice in StageParticipants")
	}

	if len(kinds) != 1 || kinds[0] != "video" {
		t.Fatalf("unexpected kinds for alice: %#v", kinds)
	}
}

func TestAgent_reconcileState(t *testing.T) {
	cases := []struct {
		name         string
		initialSnap  *RoomSnapshot
		initialSubs  []*lk.Subscription
		policy       Policy
		layout       *layout.LayoutView
		expectedSubs []*lk.Subscription
	}{
		{
			name: "subscribe to new video track",
			initialSnap: func() *RoomSnapshot {
				s := NewRoomSnapshot()
				s.UpsertParticipant("p1", "p1_identity")
				s.UpsertTrack("p1", "t1", TrackVideo)
				return s
			}(),
			initialSubs:  []*lk.Subscription{},
			policy:       Policy{IncludeKinds: []TrackKind{TrackVideo, TrackAudio}},
			layout:       func() *layout.LayoutView { v, _ := layout.NewLayoutView(""); v.StageTracks["t1"] = "p1"; return v }(),
			expectedSubs: []*lk.Subscription{{TID: "t1", PID: "p1"}},
		},
		{
			name: "do nothing for existing subscription",
			initialSnap: func() *RoomSnapshot {
				s := NewRoomSnapshot()
				s.UpsertParticipant("p1", "p1_identity")
				s.UpsertTrack("p1", "t1", TrackVideo)
				return s
			}(),
			initialSubs:  []*lk.Subscription{{TID: "t1", PID: "p1"}},
			policy:       Policy{IncludeKinds: []TrackKind{TrackVideo, TrackAudio}},
			layout:       func() *layout.LayoutView { v, _ := layout.NewLayoutView(""); v.StageTracks["t1"] = "p1"; return v }(),
			expectedSubs: []*lk.Subscription{{TID: "t1", PID: "p1"}},
		},
		{
			name: "ignore track when policy excludes its kind",
			initialSnap: func() *RoomSnapshot {
				s := NewRoomSnapshot()
				s.UpsertParticipant("p1", "p1_identity")
				s.UpsertTrack("p1", "t1", TrackAudio)
				return s
			}(),
			initialSubs:  []*lk.Subscription{},
			policy:       Policy{ExcludeKinds: []TrackKind{TrackAudio}},
			layout:       func() *layout.LayoutView { v, _ := layout.NewLayoutView(""); v.StageTracks["t1"] = "p1"; return v }(),
			expectedSubs: []*lk.Subscription{},
		},
		{
			name: "unsubscribe from track of left participant",
			initialSnap: func() *RoomSnapshot {
				s := NewRoomSnapshot()
				return s
			}(),
			initialSubs: []*lk.Subscription{{TID: "t1", PID: "p1"}},
			policy:      Policy{IncludeKinds: []TrackKind{TrackVideo, TrackAudio}},
			layout:      func() *layout.LayoutView { v, _ := layout.NewLayoutView(""); return v }(),
			// expectedOps: []Operations{&UnsubscribeOp{TrackID: "t1", PID: "p1", Reason: "not in decisions / participant left"}},
			expectedSubs: []*lk.Subscription{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			agent := NewAgent(AgentConfig{
				Logger: logging.NewNullLogger("test", "info"),
			}, tc.layout)
			mockClient := newMockLkClient()
			agent.lk = mockClient
			agent.cfg.Policy = tc.policy

			agent.roomSnap = tc.initialSnap
			mockClient.subs = tc.initialSubs
			ctx := t.Context()

			agent.reconcileState(ctx)

			if diff := cmp.Diff(tc.expectedSubs, mockClient.subs); diff != "" {
				t.Errorf("unexpected subscriptions after reconciliation (-want +got):\n%s", diff)
			}
		})
	}
}

func TestAgent_participantOnStageJoinPublishTrackLeaves(t *testing.T) {
	agent := NewAgent(AgentConfig{
		Logger: logging.NewNullLogger("test", "info"),
		Config: config.Config{
			IncludeKind: "video,audio",
		},
	}, layout.DefaultLayoutView())
	agent.trackPipelineManager = noopTrackPipelineManager{}

	agent.applyEvent(LayoutChangedEvent{
		Stage: &layout.Stage{
			Slots: []layout.Slot{
				{Identity: "alice", TrackKind: "video"},
			},
		},
	})

	agent.applyEvent(RoomChangedEvent{
		LkEvents: []lk.Event{
			lk.ParticipantJoined{PID: "p1", Identity: "alice"},
			lk.TrackPublished{PID: "p1", Identity: "alice", TID: "t1", Kind: "video"},
		},
	})

	decs := Decide(agent.cfg.Policy, agent.layoutView.Snapshot(), agent.roomSnap)
	ops := Diff(nil, decs)

	want := []Operations{
		&SubscribeOp{TrackID: "t1", PID: "p1", Action: VideoFull, Reason: "video on stage"},
	}
	if diff := cmp.Diff(want, ops); diff != "" {
		t.Fatalf("unexpected operations (-want +got):\n%s", diff)
	}

	agent.applyEvent(RoomChangedEvent{
		LkEvents: []lk.Event{
			lk.TrackSubscribed{
				PID:         "p1",
				Identity:    "alice",
				TID:         "t1",
				Kind:        "video",
				RemoteTrack: fakeRemoteTrack{id: "t1", kind: "video", ssrc: 1234},
			},
			lk.TrackUnsubscribed{PID: "p1", Identity: "alice", TID: "t1", Kind: "video"},
			lk.ParticipantLeft{PID: "p1", Identity: "alice"},
		},
	})

	if _, ok := agent.roomSnap.Participants["p1"]; ok {
		t.Fatalf("participant should be removed from snapshot")
	}

	if _, ok := agent.roomSnap.Tracks["t1"]; ok {
		t.Fatalf("track should be removed after TrackUnsubscribed")
	}

	if len(agent.roomSnap.Participants) != 0 {
		t.Fatalf("expected no participants after ParticipantLeft, got %#v", agent.roomSnap.Participants)
	}

	if len(agent.roomSnap.Tracks) != 0 {
		t.Fatalf("expected no tracks after ParticipantLeft, got %#v", agent.roomSnap.Tracks)
	}

	if _, ok := agent.roomSnap.SubCache["t1"]; ok {
		t.Fatalf("current subscription cache should be cleared for unsubscribed track")
	}

	decs = Decide(agent.cfg.Policy, agent.layoutView.Snapshot(), agent.roomSnap)
	if len(decs) != 0 {
		t.Fatalf("expected no decisions after track unsubscribe, got %#v", decs)
	}
}
