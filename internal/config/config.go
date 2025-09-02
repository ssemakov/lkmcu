package config

import (
	"flag"
	"os"
)

type Config struct {
	LivekitURL string
	APIKey     string
	APISecret  string
	LogLevel   string
	Room       string
	Identity   string
	Region     string
	Addr       string
}

func FromEnv() Config {
	return Config{
		LivekitURL: env("LIVEKIT_URL", ""),
		APIKey:     env("LIVEKIT_API_KEY", ""),
		APISecret:  env("LIVEKIT_API_SECRET", ""),
		LogLevel:   env("MCU_LOG_LEVEL", ""),
		Room:       env("LIVEKIT_ROOM", ""),
		Identity:   env("MCU_IDENTITY", ""),
		Region:     env("MCU_REGION", ""),
		Addr:       env("MCU_HTTP_ADDR", ""),
	}
}

func FromFlags() Config {
		var (
		room      = flag.String("room", "", "LiveKit room to join")
		apiKey    = flag.String("key", "", "LiveKit API key")
		apiSecret = flag.String("secret", "", "LiveKit API secret")
		identity  = flag.String("identity", "", "participant identity")
		region    = flag.String("region", "", "region tag")
		addr      = flag.String("http", "", "http listen addr")
		liveKitUrl = flag.String("url", "", "LiveKit server url")
	)
	flag.Parse()

	return Config{
		LivekitURL: *liveKitUrl,
		Room: *room,
		APIKey: *apiKey,
		APISecret: *apiSecret,
		Identity: *identity,
		Region: *region,
		Addr: *addr,
	}

}

// Resolve merges flags, env vars and defaults into a final Config
func Resolve(flags Config, env Config, defaults Config) Config {
	return Config{
		LivekitURL: firstNonEmpty(flags.LivekitURL, env.LivekitURL, defaults.LivekitURL),
		APIKey:     firstNonEmpty(flags.APIKey, env.APIKey, defaults.APIKey),
		APISecret:  firstNonEmpty(flags.APISecret, env.APISecret, defaults.APISecret),
		LogLevel:   firstNonEmpty(flags.LogLevel, env.LogLevel, defaults.LogLevel),
		Room:       firstNonEmpty(flags.Room, env.Room, defaults.Room),
		Identity:   firstNonEmpty(flags.Identity, env.Identity, defaults.Identity),
		Region:     firstNonEmpty(flags.Region, env.Region, defaults.Region),
		Addr:       firstNonEmpty(flags.Addr, env.Addr, defaults.Addr),
	}

}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
