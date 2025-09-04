package main

import (
	"context"
	"net/http"
	"os/signal"
	"syscall"

	"github.com/ssemakov/lkmcu/internal/config"
	"github.com/ssemakov/lkmcu/internal/layout"
	"github.com/ssemakov/lkmcu/internal/logging"
	"github.com/ssemakov/lkmcu/internal/metrics"
	"github.com/ssemakov/lkmcu/internal/pipeline"
)

func main() {
	cfg := config.Resolve(
		config.FromFlags(),
		config.FromEnv(),
		config.Config{ // defaults
			LivekitURL: "ws://localhost:7880",
			APIKey:     "devkey",
			APISecret:  "secret",
			LogLevel:   "info",
			Addr:       ":8080",
		},
	)

	lg := logging.New(cfg.Region, cfg.LogLevel)

	if cfg.Room == "" {
		lg.Fatalf("room is required (--room or LIVEKIT_ROOM)")
	}

	stage := layout.Default()
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200); _, _ = w.Write([]byte("ok")) })
	mux.Handle("/metrics", metrics.Handler())
	mux.HandleFunc("/layout", func(w http.ResponseWriter, r *http.Request) { layout.HTTPHandler(&stage, w, r) })

	go func() { _ = http.ListenAndServe(cfg.Addr, mux) }()
	lg.Infof("HTTP listening on %s", cfg.Addr)

	lg.Infof("starting MCU agent for room=%s identity=%s", cfg.Room, cfg.Identity)

	agent := pipeline.NewAgent(pipeline.AgentConfig{
		Layout: stage,
		Logger: lg,
		Config: cfg,
	})

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := agent.Run(ctx); err != nil {
		lg.Errorf("agent exited: %v", err)
	}
	lg.Infof("shutdown complete")
}
