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
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

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

	go a.consumeLiveKitEvents(runCtx, cancel)

	// TODO initial room state fetch
	// Get your own participant ID to be able to ignore your own events

	// Main loop
	for {
		select {
		case <-runCtx.Done():
			lg.Infof("agent stopping...")
			time.Sleep(50 * time.Millisecond)
			return runCtx.Err()

		case <-a.tick.C:
			a.reconcileState(runCtx)

		case ev := <-a.evCh:
			lg.Infof("event: %v", logging.FormatValue(ev))
			a.applyEvent(ev)
			a.reconcileState(runCtx)
		}
	}
}

// consumeLiveKitEvents streams LiveKit events into the agent's event channel until cancellation.
func (a *Agent) consumeLiveKitEvents(ctx context.Context, cancel context.CancelFunc) {
	lg := a.cfg.Logger
	for {
		select {
		case <-ctx.Done():
			return
		case e, ok := <-a.lk.Events():
			if !ok {
				lg.Infof("LiveKit event channel closed")
				return
			}
			switch evt := e.(type) {
			case lk.ParticipantJoined, lk.ParticipantLeft, lk.TrackPublished, lk.TrackUnpublished, lk.TrackSubscribed, lk.TrackUnsubscribed:
				lg.Infof("LiveKit event: %s", logging.FormatValue(e))
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
			case lk.RoomDisconnected:
				lg.Infof("Room disconnected, reason: %v", evt.Reason)
				cancel()
				return
			case lk.RoomClosed:
				lg.Infof("Room closed, stopping agent")
				cancel()
				return
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
	a.cfg.Logger.Debugf("applyEvent %v", logging.FormatValue(ev))

	switch systemEvt := ev.(type) {
	case RoomChangedEvent:
		// Update room snapshot based on the events
		for _, evt := range systemEvt.LkEvents {
			switch lkEvt := evt.(type) {
			case lk.ParticipantJoined:
				a.roomSnap.UpsertParticipant(lkEvt.PID, lkEvt.Identity)
			case lk.ParticipantLeft:

				tids := a.roomSnap.DeleteParticipantAndTracks(lkEvt.PID)
				for _, tid := range tids {
					a.roomSnap.RemoveSubCache(tid)
				}
			case lk.TrackPublished:
				_, err := a.roomSnap.UpsertTrack(lkEvt.PID, lkEvt.TID, TrackKind(lkEvt.Kind))
				if err != nil {
					a.cfg.Logger.Errorf("lk.TrackPublished error: %v", err)
					continue
				}
			case lk.TrackUnpublished:
				err := a.roomSnap.DeleteTrack(lkEvt.PID, lkEvt.TID)
				if err != nil {
					a.cfg.Logger.Errorf("lk.TrackUnpublished error: %v", err)
					continue
				}
				a.roomSnap.RemoveSubCache(lkEvt.TID)
			case lk.TrackSubscribed:
				// TODO properly convert to video-full, video-low, pause
				a.roomSnap.UpsertSubCache(lkEvt.TID, DecisionAction(lkEvt.Kind))
			case lk.TrackUnsubscribed:
				a.roomSnap.DeleteTrack(lkEvt.PID, lkEvt.TID)
				a.roomSnap.RemoveSubCache(lkEvt.TID)
			default:
				a.cfg.Logger.Debugf("ignoring LiveKit event in RoomChangedEvent: %s", logging.FormatValue(lkEvt))
			}
		}
	case LayoutChangedEvent:
		// TODO consider coasting multiple LayoutChangedEvent into one
		if systemEvt.Stage != nil {
			// Update layout view atomically
			a.layoutView.UpdateFromStage(systemEvt.Stage)
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
	// TODO cache decisions in the room snapshot
	decs := Decide(a.cfg.Policy, lSnap, a.roomSnap)
	if len(decs) != 0 {
		a.cfg.Logger.Debugf("Decisions: %v", logging.FormatValue(decs))
	}

	// Compare with current subscriptions
	// TODO cache for event reconciliation and get the full snapshot on periodic reconciliation
	curLk := a.lk.SubscriptionSnapshot()
	a.cfg.Logger.Debugf("Current LK subscriptions: %v", logging.FormatValue(curLk))

	ops := Diff(curLk, decs)
	if len(ops) != 0 {
		a.cfg.Logger.Infof("Operations to apply: %v", logging.FormatValue(ops))
	}

	for _, op := range ops {
		a.cfg.Logger.Infof("Applying subscription change: %v", logging.FormatValue(op))

		if err := op.Exec(a.lk.(*lk.Client)); err != nil {
			a.cfg.Logger.Errorf("error applying subscription change: %v", err)
		}
	}
}
