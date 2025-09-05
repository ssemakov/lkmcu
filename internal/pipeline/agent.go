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
	// layout layout.Stage // current layout
	// roomSnap RoomSnapshot // current room state

	evCh chan Event // internal events
	tick *time.Ticker // periodic safety reconciliation
}

func NewAgent(cfg AgentConfig) *Agent { return &Agent{cfg: cfg} }

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
	a.evCh = make(chan Event, 64)
	// periodic reconciliation allows for the state to be eventually consistent
	// in case we miss some events or something unexpected happens like missed or out of order events
	a.tick = time.NewTicker(5 * time.Second)
	go a.consumeLiveKitEvents(ctx)
	go a.serveLayoutApi(ctx)

	// Main loop
	for {
		select {
		case <-ctx.Done():
			lg.Infof("agent stopping...")
			time.Sleep(50 * time.Millisecond)
			return ctx.Err()

		case <-a.tick.C:
			lg.Debugf("tick: reconciling state")
			a.reconcileState(ctx)

		case ev := <-a.evCh:
			lg.Infof("event: %v", ev.Type)
			a.applyEvent(ev)
			a.reconcileState(ctx)
		case <-ctx.Done():
			break
		}
	}
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
			switch e {
				case lk.ParticipantJoined{}, lk.ParticipantLeft{}:
					lg.Infof("received LiveKit event: %v", e)
					a.evCh <- Event{Type: "LKEVent"}
				}
		default:
			// ev, err := a.lk.NextEvent(ctx)
			// lg.Infof("received LiveKit event: %v", ev)
			/// temp
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func (a *Agent) serveLayoutApi(ctx context.Context) {
	lg := a.cfg.Logger
	lg.Infof("starting layout API server")
	// TODO
	<-ctx.Done()
	lg.Infof("stopping layout API server")
}

func (a *Agent) applyEvent(ev Event) {
	a.cfg.Logger.Debugf("applying event: %v", ev)
	// TODO
}

func (a *Agent) reconcileState(ctx context.Context) {
	a.cfg.Logger.Debugf("reconciling state...")
	// TODO
}
