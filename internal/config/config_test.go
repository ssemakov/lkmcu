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
	t.Setenv("LIVEKIT_URL", "ws://env:7880")
	t.Setenv("LIVEKIT_API_KEY", "env-key")
	t.Setenv("LIVEKIT_API_SECRET", "env-secret")
	t.Setenv("MCU_LOG_LEVEL", "debug")
	t.Setenv("LIVEKIT_ROOM", "env-room")
	t.Setenv("MCU_IDENTITY", "env-identity")
	t.Setenv("MCU_REGION", "env-region")
	t.Setenv("MCU_HTTP_ADDR", ":8080")
	
	got := FromEnv()
	
	if got.LivekitURL != "ws://env:7880" {
		t.Errorf("want LivekitURL=ws://env:7880, got %q", got.LivekitURL)
	}
	if got.APIKey != "env-key" {
		t.Errorf("want APIKey=env-key, got %q", got.APIKey)
	}
	if got.APISecret != "env-secret" {
		t.Errorf("want APISecret=env-secret, got %q", got.APISecret)
	}
	if got.LogLevel != "debug" {
		t.Errorf("want LogLevel=debug, got %q", got.LogLevel)
	}
	if got.Room != "env-room" {
		t.Errorf("want Room=env-room, got %q", got.Room)
	}
	if got.Identity != "env-identity" {
		t.Errorf("want Identity=env-identity, got %q", got.Identity)
	}
	if got.Region != "env-region" {
		t.Errorf("want Region=env-region, got %q", got.Region)
	}
	if got.Addr != ":8080" {
		t.Errorf("want Addr=:8080, got %q", got.Addr)
	}
}

func TestResolve_AllFields(t *testing.T) {
	flags := Config{
		Room: "flag-room",
		APIKey: "flag-key",
	}
	env := Config{
		Room: "env-room",
		APIKey: "env-key", 
		APISecret: "env-secret",
		LogLevel: "debug",
	}
	defaults := Config{
		Room: "default-room",
		APIKey: "default-key",
		APISecret: "default-secret", 
		LogLevel: "info",
		LivekitURL: "ws://localhost:7880",
		Identity: "default-identity",
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
