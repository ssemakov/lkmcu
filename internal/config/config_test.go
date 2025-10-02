package config

import (
	"flag"
	"os"
	"testing"
)

func TestResolve_FlagOverridesEnv(t *testing.T) {
	flags := Config{Room: "flag-room"}
	env := Config{Room: "env-room"}
	defaults := Config{Room: "default-room"}

	got := Resolve(flags, env, defaults)
	if got.Room != "flag-room" {
		t.Fatalf("want flag-room, got %q", got.Room)
	}
}

func TestResolve_EnvOverridesDefault_WhenFlagEmpty(t *testing.T) {
	flags := Config{Room: ""}
	env := Config{Room: "env-room"}
	defaults := Config{Room: "default-room"}

	got := Resolve(flags, env, defaults)
	if got.Room != "env-room" {
		t.Fatalf("want env-room, got %q", got.Room)
	}
}

func TestResolve_DefaultUsed_WhenFlagAndEnvEmpty(t *testing.T) {
	flags := Config{Room: ""}
	env := Config{Room: ""}
	defaults := Config{Room: "default-room"}

	got := Resolve(flags, env, defaults)
	if got.Room != "default-room" {
		t.Fatalf("want default-room, got %q", got.Room)
	}
}

func TestFromFlags(t *testing.T) {
	// Save original os.Args and restore after test
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	// Set test arguments
	os.Args = []string{"cmd", "-room=test-room", "-key=test-key", "-url=ws://test:7880"}

	// Reset flag package for clean test
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	got := FromFlags()

	if got.Room != "test-room" {
		t.Errorf("want Room=test-room, got %q", got.Room)
	}
	if got.APIKey != "test-key" {
		t.Errorf("want APIKey=test-key, got %q", got.APIKey)
	}
	if got.LivekitURL != "ws://test:7880" {
		t.Errorf("want LivekitURL=ws://test:7880, got %q", got.LivekitURL)
	}
}

func TestFromEnv(t *testing.T) {
	os.Setenv("LIVEKIT_URL", "env-url")
	os.Setenv("LIVEKIT_API_KEY", "env-key")
	os.Setenv("LIVEKIT_API_SECRET", "env-secret")
	os.Setenv("MCU_LOG_LEVEL", "debug")
	os.Setenv("LIVEKIT_ROOM", "env-room")
	os.Setenv("MCU_IDENTITY", "env-identity")
	os.Setenv("MCU_REGION", "env-region")
	os.Setenv("MCU_HTTP_ADDR", ":9999")
	os.Setenv("MCU_INCLUDE_KIND", "audio,video")
	os.Setenv("MCU_EXCLUDE_KIND", "screen")

	defer func() {
		os.Unsetenv("LIVEKIT_URL")
		os.Unsetenv("LIVEKIT_API_KEY")
		os.Unsetenv("LIVEKIT_API_SECRET")
		os.Unsetenv("MCU_LOG_LEVEL")
		os.Unsetenv("LIVEKIT_ROOM")
		os.Unsetenv("MCU_IDENTITY")
		os.Unsetenv("MCU_REGION")
		os.Unsetenv("MCU_HTTP_ADDR")
		os.Unsetenv("MCU_INCLUDE_KIND")
		os.Unsetenv("MCU_EXCLUDE_KIND")
	}()

	cfg := FromEnv()
	if cfg.LivekitURL != "env-url" {
		t.Errorf("expected LivekitURL=env-url, got %s", cfg.LivekitURL)
	}
	if cfg.APIKey != "env-key" {
		t.Errorf("expected APIKey=env-key, got %s", cfg.APIKey)
	}
	if cfg.APISecret != "env-secret" {
		t.Errorf("expected APISecret=env-secret, got %s", cfg.APISecret)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("expected LogLevel=debug, got %s", cfg.LogLevel)
	}
	if cfg.Room != "env-room" {
		t.Errorf("expected Room=env-room, got %s", cfg.Room)
	}
	if cfg.Identity != "env-identity" {
		t.Errorf("expected Identity=env-identity, got %s", cfg.Identity)
	}
	if cfg.Region != "env-region" {
		t.Errorf("expected Region=env-region, got %s", cfg.Region)
	}
	if cfg.Addr != ":9999" {
		t.Errorf("expected Addr=:9999, got %s", cfg.Addr)
	}
	if cfg.IncludeKind != "audio,video" {
		t.Errorf("expected IncludeKind=audio,video, got %s", cfg.IncludeKind)
	}
	if cfg.ExcludeKind != "screen" {
		t.Errorf("expected ExcludeKind=screen, got %s", cfg.ExcludeKind)
	}
}

func TestResolve_AllFields(t *testing.T) {
	flags := Config{
		Room:   "flag-room",
		APIKey: "flag-key",
	}
	env := Config{
		Room:      "env-room",
		APIKey:    "env-key",
		APISecret: "env-secret",
		LogLevel:  "debug",
	}
	defaults := Config{
		Room:       "default-room",
		APIKey:     "default-key",
		APISecret:  "default-secret",
		LogLevel:   "info",
		LivekitURL: "ws://localhost:7880",
		Identity:   "default-identity",
	}

	got := Resolve(flags, env, defaults)

	// Flag values should override
	if got.Room != "flag-room" {
		t.Errorf("want Room=flag-room, got %q", got.Room)
	}
	if got.APIKey != "flag-key" {
		t.Errorf("want APIKey=flag-key, got %q", got.APIKey)
	}

	// Env should override defaults where flags are empty
	if got.APISecret != "env-secret" {
		t.Errorf("want APISecret=env-secret, got %q", got.APISecret)
	}
	if got.LogLevel != "debug" {
		t.Errorf("want LogLevel=debug, got %q", got.LogLevel)
	}

	// Defaults should be used where both flag and env are empty
	if got.LivekitURL != "ws://localhost:7880" {
		t.Errorf("want LivekitURL=ws://localhost:7880, got %q", got.LivekitURL)
	}
	if got.Identity != "default-identity" {
		t.Errorf("want Identity=default-identity, got %q", got.Identity)
	}
}

func TestResolve(t *testing.T) {
	flags := Config{
		LivekitURL:  "flag-url",
		APIKey:      "flag-key",
		APISecret:   "flag-secret",
		LogLevel:    "info",
		Room:        "flag-room",
		Identity:    "flag-identity",
		Region:      "flag-region",
		Addr:        ":8888",
		IncludeKind: "audio",
		ExcludeKind: "video",
	}
	env := Config{
		LivekitURL:  "env-url",
		APIKey:      "env-key",
		APISecret:   "env-secret",
		LogLevel:    "debug",
		Room:        "env-room",
		Identity:    "env-identity",
		Region:      "env-region",
		Addr:        ":9999",
		IncludeKind: "audio,video",
		ExcludeKind: "screen",
	}
	def := Config{
		LivekitURL:  "def-url",
		APIKey:      "def-key",
		APISecret:   "def-secret",
		LogLevel:    "error",
		Room:        "def-room",
		Identity:    "def-identity",
		Region:      "def-region",
		Addr:        ":7777",
		IncludeKind: "video",
		ExcludeKind: "",
	}

	cfg := Resolve(flags, env, def)
	if cfg.LivekitURL != "flag-url" {
		t.Errorf("expected LivekitURL=flag-url, got %s", cfg.LivekitURL)
	}
	if cfg.APIKey != "flag-key" {
		t.Errorf("expected APIKey=flag-key, got %s", cfg.APIKey)
	}
	if cfg.APISecret != "flag-secret" {
		t.Errorf("expected APISecret=flag-secret, got %s", cfg.APISecret)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("expected LogLevel=info, got %s", cfg.LogLevel)
	}
	if cfg.Room != "flag-room" {
		t.Errorf("expected Room=flag-room, got %s", cfg.Room)
	}
	if cfg.Identity != "flag-identity" {
		t.Errorf("expected Identity=flag-identity, got %s", cfg.Identity)
	}
	if cfg.Region != "flag-region" {
		t.Errorf("expected Region=flag-region, got %s", cfg.Region)
	}
	if cfg.Addr != ":8888" {
		t.Errorf("expected Addr=:8888, got %s", cfg.Addr)
	}
	if cfg.IncludeKind != "audio" {
		t.Errorf("expected IncludeKind=audio, got %s", cfg.IncludeKind)
	}
	if cfg.ExcludeKind != "video" {
		t.Errorf("expected ExcludeKind=video, got %s", cfg.ExcludeKind)
	}
}
