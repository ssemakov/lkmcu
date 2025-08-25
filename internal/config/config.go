package config

import "os"

type Config struct {
	LivekitURL string
	APIKey     string
	APISecret  string
	LogLevel   string
}

func FromEnv() Config {
	return Config{
		LivekitURL: env("LIVEKIT_URL", "ws://localhost:7880"),
		APIKey:     env("LIVEKIT_API_KEY", ""),
		APISecret:  env("LIVEKIT_API_SECRET", ""),
		LogLevel:   env("MCU_LOG_LEVEL", "info"),
	}
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
