package config

import (
	"crypto/rand"
	"flag"
	"os"
	"time"

	"github.com/oklog/ulid/v2"
)

type Config struct {
	LivekitURL  string
	APIKey      string
	APISecret   string
	LogLevel    string
	Room        string
	Identity    string
	Region      string
	Addr        string
	IncludeKind string
	ExcludeKind string
}

func FromEnv() Config {
	return Config{
		LivekitURL:  env("LIVEKIT_URL", ""),
		APIKey:      env("LIVEKIT_API_KEY", ""),
		APISecret:   env("LIVEKIT_API_SECRET", ""),
		LogLevel:    env("MCU_LOG_LEVEL", ""),
		Room:        env("LIVEKIT_ROOM", ""),
		Identity:    env("MCU_IDENTITY", ""),
		Region:      env("MCU_REGION", ""),
		Addr:        env("MCU_HTTP_ADDR", ""),
		IncludeKind: env("MCU_INCLUDE_KIND", ""),
		ExcludeKind: env("MCU_EXCLUDE_KIND", ""),
	}
}

func FromFlags() Config {
	var (
		room        = flag.String("room", "", "LiveKit room to join")
		apiKey      = flag.String("key", "", "LiveKit API key")
		apiSecret   = flag.String("secret", "", "LiveKit API secret")
		identity    = flag.String("identity", "", "participant identity")
		region      = flag.String("region", "", "region tag")
		addr        = flag.String("http", "", "http listen addr")
		liveKitUrl  = flag.String("url", "", "LiveKit server url")
		logLevel    = flag.String("log-level", "", "log level, options: debug, info (default), error")
		includeKind = flag.String("include-kind", "", "comma-separated list of track kinds to include (empty = all), options: audio, video, screen")
		excludeKind = flag.String("exclude-kind", "", "comma-separated list of track kinds to exclude, options: audio, video, screen")
	)
	flag.Parse()

	return Config{
		LivekitURL:  *liveKitUrl,
		Room:        *room,
		APIKey:      *apiKey,
		APISecret:   *apiSecret,
		Identity:    *identity,
		Region:      *region,
		Addr:        *addr,
		LogLevel:    *logLevel,
		IncludeKind: *includeKind,
		ExcludeKind: *excludeKind,
	}

}

// Resolve merges flags, env vars and defaults into a final Config
func Resolve(flags Config, env Config, defaults Config) Config {
	return Config{
		LivekitURL:  firstNonEmpty(flags.LivekitURL, env.LivekitURL, defaults.LivekitURL),
		APIKey:      firstNonEmpty(flags.APIKey, env.APIKey, defaults.APIKey),
		APISecret:   firstNonEmpty(flags.APISecret, env.APISecret, defaults.APISecret),
		LogLevel:    firstNonEmpty(flags.LogLevel, env.LogLevel, defaults.LogLevel),
		Room:        firstNonEmpty(flags.Room, env.Room, defaults.Room),
		Identity:    firstNonEmpty(flags.Identity, env.Identity, defaults.Identity, "Ex_"+NewID(12)),
		Region:      firstNonEmpty(flags.Region, env.Region, defaults.Region),
		Addr:        firstNonEmpty(flags.Addr, env.Addr, defaults.Addr),
		IncludeKind: firstNonEmpty(flags.IncludeKind, env.IncludeKind, defaults.IncludeKind),
		ExcludeKind: firstNonEmpty(flags.ExcludeKind, env.ExcludeKind, defaults.ExcludeKind),
	}

}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func NewID(n int) string {
	t := time.Now()
	entropy := ulid.Monotonic(rand.Reader, 0)

	id := ulid.MustNew(ulid.Timestamp(t), entropy)
	data := id.Bytes()

	out := make([]byte, n)
	for i := range out {
		out[i] = alphabet[int(data[i])%len(alphabet)]
	}

	return string(out)
}
