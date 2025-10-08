package pipeline

import "strings"

// Subscription policy, allows to include/exclude certain kinds of tracks
// and limit the number of video decoders
// Should be set by the outside configuration
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
	for _, k := range p.IncludeKinds {
		if k == kind {
			return true
		}
	}
	return false
}

func (p *Policy) IsExcluded(kind TrackKind) bool {
	for _, k := range p.ExcludeKinds {
		if k == kind {
			return true
		}
	}
	return false
}
