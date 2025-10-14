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

	lg := logging.NewConsoleLogger(cfg.Region, cfg.LogLevel)

	if cfg.Room == "" {
		lg.Fatalf("room is required (--room or LIVEKIT_ROOM)")
	}

	lView, err := layout.NewLayoutView(cfg.IntialStage)
	if err != nil {
		lg.Fatalf("invalid initial stage layout: %v", err)
	} else {
		lg.Infof("loaded initial stage layout: %v", logging.FormatValue(lView.CurrentStage))
	}

	agent := pipeline.NewAgent(pipeline.AgentConfig{
		Logger: lg,
		Config: cfg,
	}, lView)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200); _, _ = w.Write([]byte("ok")) })
	mux.Handle("/metrics", metrics.Handler())
	mux.HandleFunc("/layout", func(w http.ResponseWriter, r *http.Request) {
		// For GET, capture a consistent pointer snapshot to avoid a data race
		stagePtr := lView.Snapshot().CurrentStage
		err := layout.HTTPHandler(stagePtr, func(st *layout.Stage) {
			agent.OnLayoutStageUpdate(st)
		}, w, r)
		if err != nil {
			lg.Errorf("layout handler error: %v", err)
		}
	})

	go func() { _ = http.ListenAndServe(cfg.Addr, mux) }()
	lg.Infof("HTTP listening on %s", cfg.Addr)

	lg.Infof("starting MCU agent for room=%s identity=%s", cfg.Room, cfg.Identity)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := agent.Run(ctx); err != nil {
		lg.Errorf("agent exited: %v", err)
	}
	lg.Infof("shutdown complete")
}
