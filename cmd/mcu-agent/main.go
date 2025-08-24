package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"livekit-mcu/internal/config"
	"livekit-mcu/internal/layout"
	"livekit-mcu/internal/logging"
	"livekit-mcu/internal/metrics"
	"livekit-mcu/internal/pipeline"
)

func main() {
	var (
		room     = flag.String("room", os.Getenv("LIVEKIT_ROOM"), "LiveKit room to join")
		identity = flag.String("identity", "mcu-bot", "participant identity")
		region   = flag.String("region", os.Getenv("MCU_REGION"), "region tag")
		addr     = flag.String("http", ":8080", "http listen addr")
	)
	flag.Parse()

	cfg := config.FromEnv()
	lg := logging.New(*region, cfg.LogLevel)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	stage := layout.Default()
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200); _, _ = w.Write([]byte("ok")) })
	mux.Handle("/metrics", metrics.Handler())
	mux.HandleFunc("/layout", func(w http.ResponseWriter, r *http.Request) { layout.HTTPHandler(&stage, w, r) })

	go func() { _ = http.ListenAndServe(*addr, mux) }()
	lg.Infof("HTTP listening on %s", *addr)

	if *room == "" { lg.Fatalf("room is required (--room or LIVEKIT_ROOM)") }
	lg.Infof("starting MCU agent for room=%s identity=%s", *room, *identity)

	agent := pipeline.NewAgent(pipeline.AgentConfig{
		Room:     *room,
		Identity: *identity,
		Layout:   stage,
		Logger:   lg,
		Config:   cfg,
	})
	if err := agent.Run(ctx); err != nil {
		lg.Errorf("agent exited: %v", err)
	}
	lg.Infof("shutdown complete")
}
