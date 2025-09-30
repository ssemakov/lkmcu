package pipeline

import (
	"testing"
)

func TestPolicy_IsIncluded(t *testing.T) {
	p := &Policy{
		IncludeKinds: []TrackKind{TrackVideo, TrackAudio},
	}

	if !p.IsIncluded(TrackVideo) {
		t.Errorf("expected TrackVideo to be included")
	}
	if !p.IsIncluded(TrackAudio) {
		t.Errorf("expected TrackAudio to be included")
	}
	if p.IsIncluded(TrackScreenShare) {
		t.Errorf("expected TrackScreenShare to NOT be included")
	}

	// If IncludeKinds is empty, all should be included
	p2 := &Policy{}
	if !p2.IsIncluded(TrackVideo) || !p2.IsIncluded(TrackAudio) || !p2.IsIncluded(TrackScreenShare) {
		t.Errorf("expected all kinds to be included when IncludeKinds is empty")
	}
}

func TestPolicy_IsExcluded(t *testing.T) {
	p := &Policy{
		ExcludeKinds: []TrackKind{TrackScreenShare},
	}

	if !p.IsExcluded(TrackScreenShare) {
		t.Errorf("expected TrackScreenShare to be excluded")
	}
	if p.IsExcluded(TrackVideo) {
		t.Errorf("expected TrackVideo to NOT be excluded")
	}
	if p.IsExcluded(TrackAudio) {
		t.Errorf("expected TrackAudio to NOT be excluded")
	}

	// If ExcludeKinds is empty, none should be excluded
	p2 := &Policy{}
	if p2.IsExcluded(TrackVideo) || p2.IsExcluded(TrackAudio) || p2.IsExcluded(TrackScreenShare) {
		t.Errorf("expected no kinds to be excluded when ExcludeKinds is empty")
	}
}
