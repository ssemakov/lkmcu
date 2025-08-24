package pipeline

import (
	"context"
	"time"
	"lkmcu/internal/config"
	"lkmcu/internal/layout"
	"lkmcu/internal/logging"
)

type AgentConfig struct {
	Room     string
	Identity string
	Layout   layout.Stage
	Logger   *logging.Logger
	Config   config.Config
}

type Agent struct{ cfg AgentConfig }

func NewAgent(cfg AgentConfig) *Agent { return &Agent{cfg: cfg} }

func (a *Agent) Run(ctx context.Context) error {
	lg := a.cfg.Logger
	lg.Infof("agent starting for room=%s identity=%s", a.cfg.Room, a.cfg.Identity)
	<-ctx.Done()
	lg.Infof("agent stopping...")
	time.Sleep(50 * time.Millisecond)
	return ctx.Err()
}
