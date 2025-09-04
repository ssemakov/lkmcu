package pipeline

import (
	"context"
	"testing"
	"time"

	"github.com/ssemakov/lkmcu/internal/config"
	"github.com/ssemakov/lkmcu/internal/layout"
	"github.com/ssemakov/lkmcu/internal/logging"
)

func TestNewAgent(t *testing.T) {
	cfg := AgentConfig{
		Layout: layout.Default(),
		Logger: logging.New("test", "info"),
		Config: config.Config{
			Room:       "test-room",
			Identity:   "test-identity",
			LivekitURL: "ws://localhost:7880",
			APIKey:     "test-key",
			APISecret:  "test-secret",
		},
	}

	agent := NewAgent(cfg)

	if agent == nil {
		t.Fatal("NewAgent returned nil")
	}
	if agent.cfg.Config.Room != "test-room" {
		t.Errorf("want Room=test-room, got %s", agent.cfg.Config.Room)
	}
	if agent.cfg.Config.Identity != "test-identity" {
		t.Errorf("want Identity=test-identity, got %s", agent.cfg.Config.Identity)
	}
}

func TestNewAgent_EmptyConfig(t *testing.T) {
	cfg := AgentConfig{}
	agent := NewAgent(cfg)

	if agent == nil {
		t.Fatal("NewAgent returned nil")
	}
	if agent.cfg.Logger != nil {
		t.Error("expected nil logger in empty config")
	}
}

func TestAgent_Run_ContextCancellation(t *testing.T) {
	cfg := AgentConfig{
		Layout: layout.Default(),
		Logger: logging.New("test", "info"),
		Config: config.Config{
			Room:       "test-room",
			Identity:   "test-identity",
			LivekitURL: "ws://localhost:7880",
			APIKey:     "test-key",
			APISecret:  "test-secret",
		},
	}

	agent := NewAgent(cfg)
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel context immediately to test cancellation behavior
	cancel()

	err := agent.Run(ctx)
	if err != context.Canceled {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
}

func TestAgent_Run_ContextTimeout(t *testing.T) {
	cfg := AgentConfig{
		Layout: layout.Default(),
		Logger: logging.New("test", "info"),
		Config: config.Config{
			Room:       "test-room",
			Identity:   "test-identity",
			LivekitURL: "ws://localhost:7880",
			APIKey:     "test-key",
			APISecret:  "test-secret",
		},
	}

	agent := NewAgent(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := agent.Run(ctx)
	duration := time.Since(start)

	if err != context.DeadlineExceeded {
		t.Errorf("expected context.DeadlineExceeded error, got %v", err)
	}

	// Should take approximately 100ms + 50ms sleep
	if duration < 100*time.Millisecond || duration > 200*time.Millisecond {
		t.Errorf("expected duration around 150ms, got %v", duration)
	}
}

func TestAgent_Run_DifferentConfigs(t *testing.T) {
	tests := []struct {
		name   string
		config config.Config
	}{
		{
			name: "minimal config",
			config: config.Config{
				Room:     "room1",
				Identity: "user1",
			},
		},
		{
			name: "full config",
			config: config.Config{
				Room:       "room2",
				Identity:   "user2",
				LivekitURL: "wss://example.com",
				APIKey:     "key123",
				APISecret:  "secret456",
				Region:     "us-east",
			},
		},
		{
			name: "empty strings",
			config: config.Config{
				Room:       "",
				Identity:   "",
				LivekitURL: "",
				APIKey:     "",
				APISecret:  "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := AgentConfig{
				Layout: layout.Default(),
				Logger: logging.New("test", "info"),
				Config: tt.config,
			}

			agent := NewAgent(cfg)
			ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
			defer cancel()

			// Should not panic regardless of config values
			err := agent.Run(ctx)
			if err != context.DeadlineExceeded {
				t.Errorf("expected context.DeadlineExceeded error, got %v", err)
			}
		})
	}
}

func TestAgent_Run_NilLogger(t *testing.T) {
	cfg := AgentConfig{
		Layout: layout.Default(),
		Logger: nil, // nil logger
		Config: config.Config{
			Room:     "test-room",
			Identity: "test-identity",
		},
	}

	agent := NewAgent(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Should panic due to nil logger, so we recover from the panic
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic due to nil logger, but didn't panic")
		}
	}()

	agent.Run(ctx)
}

func TestAgentConfig_Validation(t *testing.T) {
	// Test that AgentConfig struct accepts all expected types
	logger := logging.New("test", "debug")
	layout := layout.Default()
	config := config.Config{
		Room:     "test",
		Identity: "test",
	}

	cfg := AgentConfig{
		Layout: layout,
		Logger: logger,
		Config: config,
	}

	if len(cfg.Layout.Slots) != 4 {
		t.Errorf("expected 4 layout slots, got %d", len(cfg.Layout.Slots))
	}
	if cfg.Logger == nil {
		t.Error("logger should not be nil")
	}
	if cfg.Config.Room != "test" {
		t.Errorf("expected Room=test, got %s", cfg.Config.Room)
	}
}
