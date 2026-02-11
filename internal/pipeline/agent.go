package pipeline

import (
	"context"
	"time"

	"github.com/ssemakov/lkmcu/internal/config"
	"github.com/ssemakov/lkmcu/internal/layout"
	lk "github.com/ssemakov/lkmcu/internal/livekit"
	"github.com/ssemakov/lkmcu/internal/logging"
)

// AgentConfig collects dependencies and settings needed to start an Agent.
type AgentConfig struct {
	Logger logging.Logger
	Config config.Config
	Policy Policy
}

type lkClient interface {
	JoinRoom(roomName, identity string) error
	Close() error
	Events() <-chan lk.Event
	SubscriptionSnapshot() []*lk.Subscription
}

// Agent coordinates LiveKit events and layout updates to keep subscriptions in sync.
type Agent struct {
	cfg AgentConfig
	lk  lkClient

	layoutView *layout.LayoutView // current layout view
	roomSnap   *RoomSnapshot      // current room state

	evCh chan SystemEvent // internal events
	tick *time.Ticker     // periodic safety reconciliation
}

// NewAgent constructs an Agent with policy initialized from the provided configuration.
func NewAgent(cfg AgentConfig, l *layout.LayoutView) *Agent {
	cfg.Policy = NewPolicy(cfg.Config.IncludeKind, cfg.Config.ExcludeKind)

	return &Agent{
		cfg:        cfg,
		roomSnap:   NewRoomSnapshot(), // room snapshot is single-threaded, no locks
		layoutView: l,
		evCh:       make(chan SystemEvent, 64),
	}
}

// OnLayoutStageUpdate pushes a LayoutChangedEvent reflecting the supplied stage into the agent loop.
func (a *Agent) OnLayoutStageUpdate(s *layout.Stage) {
	a.evCh <- LayoutChangedEvent{Stage: s}
}

// Run connects to LiveKit and drives reconciliation until the context is cancelled.
func (a *Agent) Run(ctx context.Context) error {
	lg := a.cfg.Logger
	lg.Infof("agent starting for room=%s identity=%s", a.cfg.Config.Room, a.cfg.Config.Identity)
	lg.Infof("connecting to LiveKit at %s", a.cfg.Config.LivekitURL)

	// Join the livekit room
	a.lk = lk.NewClient(a.cfg.Config.LivekitURL, a.cfg.Config.APIKey, a.cfg.Config.APISecret, nil)
	if err := a.lk.JoinRoom(a.cfg.Config.Room, a.cfg.Config.Identity); err != nil {
		lg.Fatalf("could not join room: %v", err)
	} else {
		lg.Infof("joined room %s as %s", a.cfg.Config.Room, a.cfg.Config.Identity)
	}

	defer func() {
		if err := a.lk.Close(); err != nil {
			lg.Errorf("error closing LiveKit client: %v", err)
		}
	}()

	// periodic reconciliation allows for the state to be eventually consistent
	// in case we miss some events or something unexpected happens like missed or out of order events
	a.tick = time.NewTicker(5 * time.Second)
	defer a.tick.Stop()

	go a.consumeLiveKitEvents(ctx)

	// TODO initial room state fetch
	// Get your own participant ID to be able to ignore your own events

	// Main loop
	for {
		select {
		case <-ctx.Done():
			lg.Infof("agent stopping...")
			time.Sleep(50 * time.Millisecond)
			return ctx.Err()

		case <-a.tick.C:
			a.reconcileState(ctx)

		case ev := <-a.evCh:
			lg.Infof("event: %v", logging.FormatValue(ev))
			a.applyEvent(ev)
			a.reconcileState(ctx)
		}
	}
}

// consumeLiveKitEvents streams LiveKit events into the agent's event channel until cancellation.
func (a *Agent) consumeLiveKitEvents(ctx context.Context) {
	lg := a.cfg.Logger
	lg.Debugf("starting LiveKit event consumer")
	for {
		select {
		case <-ctx.Done():
			lg.Debugf("stopping LiveKit event consumer")
			return
		case e, ok := <-a.lk.Events():
			if !ok {
				lg.Infof("LiveKit event channel closed")
				return
			}
			switch evt := e.(type) {
			case lk.ParticipantJoined, lk.ParticipantLeft, lk.TrackPublished, lk.TrackUnpublished:
				lg.Infof("received LiveKit event: %s", logging.FormatValue(e))
				a.evCh <- RoomChangedEvent{LkEvents: []lk.Event{e}}
				// TODO coalesce multiple ParticipantJoined/Left, TrackPublished/Unpublished events into one RoomChangedEvent
				// The idea is to pull as many events as possible from the channel without blocking
				// and then push a single RoomChangedEvent to the event channel.
				// This way we avoid excessive reconciliations when many participants join/leave or publish/unpublish tracks at once.
				// Something like this:
				// for {
				// 	select {
				// 	case e2 := <-a.lk.Events():
				// 		switch e2.(type) {
				// 		case lk.ParticipantJoined, lk.ParticipantLeft, lk.TrackPublished, lk.TrackUnpublished:
				// 			lg.Infof("coalescing LiveKit event: %s", logging.FormatValue(e2))
				// 		default:
				// 			// push back the event for later processing
				// 			a.lk.PushBackEvent(e2)
				// 			break
				// 		}
				// 	default:
				// 		break
				// 	}
				// }
			case lk.TrackSubscribed:
				lg.Infof("TrackSubscribed: %s track %s kind %s", evt.Identity, evt.TID, evt.Kind)
				// a.onStageTracks[evt.TID] = true // assume subscribed tracks are on stage
			// todo ROOM_CLOSED
			default:
				lg.Debugf("ignoring LiveKit event: %s", logging.FormatValue(evt))
			}
		default:
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// applyEvent updates the room snapshot or layout view based on incoming system events.
func (a *Agent) applyEvent(ev SystemEvent) {
	a.cfg.Logger.Debugf("applying event: %v", logging.FormatValue(ev))

	switch sysE := ev.(type) {
	case RoomChangedEvent:
		// Update room snapshot based on the events
		for _, lkEv := range sysE.LkEvents {
			switch lkE := lkEv.(type) {
			case lk.ParticipantJoined:
				a.roomSnap.UpsertParticipant(lkE.PID, lkE.Identity)
			case lk.ParticipantLeft:
				a.roomSnap.DeleteParticipantAndTracks(lkE.PID)
				// TODO also unsubscribe from all tracks of this participant w/o waiting for reconciliation
				// a.roomSnap.RemoveSubsCache(tids)
			case lk.TrackPublished:
				_, err := a.roomSnap.UpsertTrack(lkE.PID, lkE.TID, TrackKind(lkE.Kind))
				if err != nil {
					a.cfg.Logger.Errorf("TrackPublished error: %v", err)
					continue
				}
			case lk.TrackUnpublished:
				err := a.roomSnap.DeleteTrack(lkE.PID, lkE.TID)
				// a.roomSnap.RemoveSubsCache(lkE.TID)
				if err != nil {
					a.cfg.Logger.Errorf("TrackUnpublished error: %v", err)
					continue
				}
			default:
				a.cfg.Logger.Debugf("ignoring LiveKit event in RoomChangedEvent: %s", logging.FormatValue(lkE))
			}
		}
	case LayoutChangedEvent:
		// TODO consider coasting multiple LayoutChangedEvent into one
		if sysE.Stage != nil {
			// Update layout view atomically
			a.layoutView.UpdateFromStage(sysE.Stage)
		}
		a.cfg.Logger.Debugf("updated stage layout: %v", logging.FormatValue(a.layoutView.CurrentStage))
	default:
		a.cfg.Logger.Errorf("unknown event type: %T", ev)
	}
}

// reconcileState decides and logs the subscription operations required to match desired layout state.
func (a *Agent) reconcileState(ctx context.Context) {
	// Decide changes based on the current layout and policy
	a.cfg.Logger.Debugf("Reconciling state. Room snapshot: %v", logging.FormatValue(a.roomSnap))

	// Take a snapshot of the layout to avoid holding locks during Decide
	lSnap := a.layoutView.Snapshot()

	// TODO we may want to add PartialDecide only for dirty set of PID, TID (ones that were added/removed/changed)
	// This can be done after profiler says that Decide is a bottleneck
	// or we can keep track of dirty PIDs/TIDs in the room snapshot and pass them to Decide
	// For now we just call full Decide every time
	// TODO cache decisions on the room snapshot
	decs := Decide(a.cfg.Policy, lSnap, a.roomSnap)
	if len(decs) != 0 {
		a.cfg.Logger.Infof("New decisions: %v", logging.FormatValue(decs))
	}

	// Compare with current subscriptions
	// TODO cache for event reconciliation and get the full snapshot on periodic reconciliation
	cur := a.lk.SubscriptionSnapshot()
	a.cfg.Logger.Debugf("Current subscriptions: %v", logging.FormatValue(cur))

	ops := Diff(cur, decs)
	if len(ops) != 0 {
		a.cfg.Logger.Infof("Operations: %v", logging.FormatValue(ops))
	}

	for _, op := range ops {
		a.cfg.Logger.Infof("applying subscription change: %v", logging.FormatValue(op))
		// TODO handle error
		op.Exec(a.lk.(*lk.Client))
	}
}
