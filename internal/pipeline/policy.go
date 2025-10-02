package pipeline

// Subscription policy, allows to include/exclude certain kinds of tracks
// and limit the number of video decoders
// Should be set by the outside configuration
type Policy struct {
	IncludeKinds     []TrackKind
	ExcludeKinds     []TrackKind
	MaxVideoDecoders int
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
