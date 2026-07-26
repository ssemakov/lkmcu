package pipeline

import (
	"slices"
	"strings"
)

// Policy allows to include/exclude certain kinds of tracks from a subscription
// and limit the number of video decoders
type Policy struct {
	IncludeKinds     []TrackKind
	ExcludeKinds     []TrackKind
	MaxVideoDecoders int
}

func NewPolicy(includeKind, excludedKind string) Policy {
	incKinds := parseTrackKinds(includeKind)
	exclKinds := parseTrackKinds(excludedKind)

	return Policy{
		IncludeKinds:     incKinds,
		ExcludeKinds:     exclKinds,
		MaxVideoDecoders: 1, // configure one video decoder by default
	}
}

func parseTrackKinds(s string) []TrackKind {
	if s == "" {
		return nil
	}
	parts := make([]TrackKind, 0)
	for _, p := range splitAndTrim(s, ",") {
		if k, err := ParseTrackKind(p); err == nil {
			parts = append(parts, k)
		} else {
			panic(err)
		}
	}
	return parts
}

func splitAndTrim(s, sep string) []string {
	parts := strings.Split(s, sep)
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func (p *Policy) IsIncluded(kind TrackKind) bool {
	if len(p.IncludeKinds) == 0 {
		return true
	}
	return slices.Contains(p.IncludeKinds, kind)
}

func (p *Policy) IsExcluded(kind TrackKind) bool {
	return slices.Contains(p.ExcludeKinds, kind)
}
