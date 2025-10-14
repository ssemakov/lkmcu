package pipeline

import (
	"github.com/ssemakov/lkmcu/internal/layout"
	"github.com/ssemakov/lkmcu/internal/livekit"
)

type EventType string

type SystemEvent interface{ isSystemEvent() }

type (
	RoomChangedEvent struct {
		LkEvents []livekit.Event // can contain multiple coalesced events
	}
	// LayoutChangedEvent carries a new Stage; Agent updates its LayoutView.
	LayoutChangedEvent struct{ Stage *layout.Stage }
)

func (RoomChangedEvent) isSystemEvent()   {}
func (LayoutChangedEvent) isSystemEvent() {}
