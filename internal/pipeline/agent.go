package pipeline

import (
	"context"
	"time"

	"github.com/ssemakov/lkmcu/internal/config"
	"github.com/ssemakov/lkmcu/internal/layout"
	lk "github.com/ssemakov/lkmcu/internal/livekit"
	"github.com/ssemakov/lkmcu/internal/logging"
)

type AgentConfig struct {
	Layout layout.Stage
	Logger *logging.Logger
	Config config.Config
}
type Agent struct{
	cfg AgentConfig
	lk *lk.Client

	// policy Policy // tracks subscription policy
	// stageTracks map[string]bool // track ID -> onStage

	layoutView *layout.LayoutView // current layout view
	roomSnap *RoomSnapshot // current room state

	evCh chan SystemEvent // internal events
	tick *time.Ticker // periodic safety reconciliation
}

func NewAgent(cfg AgentConfig) *Agent {
	return &Agent{
		cfg: cfg,
		roomSnap: NewRoomSnapshot(),
		layoutView: layout.NewLayoutView(),
	}
}

func (a *Agent) Run(ctx context.Context) error {
	lg := a.cfg.Logger
	lg.Infof("agent starting for room=%s identity=%s", a.cfg.Config.Room, a.cfg.Config.Identity)
	lg.Infof("connecting to LiveKit at %s", a.cfg.Config.LivekitURL)

	// Join the livekit room
	a.lk = lk.NewClient(a.cfg.Config.LivekitURL, a.cfg.Config.APIKey, a.cfg.Config.APISecret, nil)
	if err := a.lk.JoinRoom(a.cfg.Config.Room, a.cfg.Config.Identity); err != nil {
		lg.Errorf("could not join room: %v", err)
	} else {
		lg.Infof("joined room %s as %s", a.cfg.Config.Room, a.cfg.Config.Identity)
	}

	defer func() {
		if err := a.lk.Close(); err != nil {
			lg.Errorf("error closing LiveKit client: %v", err)
		}
	}()

	// Wire up event handlers
	a.evCh = make(chan SystemEvent, 64)
	// periodic reconciliation allows for the state to be eventually consistent
	// in case we miss some events or something unexpected happens like missed or out of order events
	a.tick = time.NewTicker(5 * time.Second)
	go a.consumeLiveKitEvents(ctx)
	go a.serveLayoutApi(ctx)

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
			// TODO query LiveKit for the current room state and compare with our state
			a.reconcileState(ctx)

		case ev := <-a.evCh:
			lg.Infof("event: %v", logging.FormatValue(ev))
			a.applyEvent(ev)
			a.reconcileState(ctx)
		case <-ctx.Done():
			break
		}
	}
}

func (a *Agent) serveLayoutApi(ctx context.Context) {
	lg := a.cfg.Logger
	lg.Infof("starting layout API server")
	// TODO
	// push layout changes to the event channel
	<-ctx.Done()
	lg.Infof("stopping layout API server")
}

func (a *Agent) consumeLiveKitEvents(ctx context.Context) {
	lg := a.cfg.Logger
	lg.Debugf("starting LiveKit event consumer")
	for {
		select {
		case <-ctx.Done():
			lg.Debugf("stopping LiveKit event consumer")
			return
		case e := <-a.lk.Events():
			switch e.(type) {
				case lk.ParticipantJoined, lk.ParticipantLeft, lk.TrackPublished, lk.TrackUnpublished:
					lg.Infof("received LiveKit event: %s", logging.FormatValue(e))
					a.evCh <- RoomChangedEvent{ LkEvents: []lk.Event{e} }
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
					ev := e.(lk.TrackSubscribed)
					lg.Infof("TrackSubscribed: %s track %s kind %s", ev.Identity, ev.TID, ev.Kind)
					// a.onStageTracks[ev.TID] = true // assume subscribed tracks are on stage
				default:
					lg.Debugf("ignoring LiveKit event: %s", logging.FormatValue(e))
				}
		default:
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func (a *Agent) applyEvent(ev SystemEvent) {
	a.cfg.Logger.Debugf("applying event: %v", logging.FormatValue(ev))

	switch e := ev.(type) {
	case RoomChangedEvent:
		// Update room snapshot based on the events
		for _, lkEv := range e.LkEvents {
			switch ev2 := lkEv.(type) {
			case lk.ParticipantJoined:
				a.roomSnap.UpsertParticipant(ev2.PID, ev2.Identity)
			case lk.ParticipantLeft:
				a.roomSnap.DeleteParticipant(ev2.PID)
			case lk.TrackPublished:
				_, err := a.roomSnap.UpsertTrack(ev2.PID, ev2.TID, TrackKind(ev2.Kind))
				if err != nil {
					a.cfg.Logger.Errorf("TrackPublished error: %v", err)
					continue
				}
			case lk.TrackUnpublished:
				a.roomSnap.DeleteTrack(ev2.PID, ev2.TID)
			default:
				a.cfg.Logger.Debugf("ignoring LiveKit event in RoomChangedEvent: %s", logging.FormatValue(ev2))
			}
		}
	case LayoutChangedEvent:
		// Update room layout snapshot
	}
}

func (a *Agent) reconcileState(ctx context.Context) {
	// Decide changes based on the current layout and policy
	a.cfg.Logger.Debugf("Reconciling state. Room snapshot: %v", logging.FormatValue(a.roomSnap))

	// TODO we may want to add PartialDecide only for dirty set of PID, TID (ones that were added/removed/changed)
	// This can be done after profiler says that Decide is a bottleneck
	// or we can keep track of dirty PIDs/TIDs in the room snapshot and pass them to Decide
	// For now we just call full Decide every time
	// TODO cache decisions on the room snapshot
	decs := Decide(&Policy{}, a.layoutView, a.roomSnap)
	a.cfg.Logger.Infof("Decisions: %v", logging.FormatValue(decs))

	// Compare with current subscriptions
	// TODO cache for event reconciliation and get the full sanpshot on periodic reconciliation
	cur := a.lk.SubscriptionSnapshot()
	a.cfg.Logger.Infof("Current subscriptions: %v", logging.FormatValue(cur))

	ops := Diff(cur, decs)
	a.cfg.Logger.Infof("Operations: %v", logging.FormatValue(ops))

	for _, op := range ops {
		a.cfg.Logger.Infof("applying subscription change: %v", logging.FormatValue(op))
		// TODO apply the change via LiveKit client
	}
}

