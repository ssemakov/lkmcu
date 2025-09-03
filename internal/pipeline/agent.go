package pipeline

import (
	"context"
	"time"

	"github.com/ssemakov/lkmcu/internal/config"
	"github.com/ssemakov/lkmcu/internal/layout"
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
	<-ctx.Done()
	lg.Infof("agent stopping...")
	time.Sleep(50 * time.Millisecond)
	return ctx.Err()
}

