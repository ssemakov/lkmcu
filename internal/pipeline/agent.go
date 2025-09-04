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

type Agent struct{ cfg AgentConfig }

func NewAgent(cfg AgentConfig) *Agent { return &Agent{cfg: cfg} }

func (a *Agent) Run(ctx context.Context) error {
	lg := a.cfg.Logger
	lg.Infof("agent starting for room=%s identity=%s", a.cfg.Config.Room, a.cfg.Config.Identity)
	lg.Infof("connecting to LiveKit at %s", a.cfg.Config.LivekitURL)

	lkClient := lk.NewClient(a.cfg.Config.LivekitURL, a.cfg.Config.APIKey, a.cfg.Config.APISecret)
	if err := lkClient.JoinRoom(a.cfg.Config.Room, a.cfg.Config.Identity); err != nil {
		lg.Errorf("could not join room: %v", err)
	} else {
		lg.Infof("joined room %s as %s", a.cfg.Config.Room, a.cfg.Config.Identity)
	}

	defer func() {
		if err := lkClient.Close(); err != nil {
			lg.Errorf("error closing LiveKit client: %v", err)
		}
	}()

	<-ctx.Done()

	lg.Infof("agent stopping...")
	time.Sleep(50 * time.Millisecond)
	return ctx.Err()
}
