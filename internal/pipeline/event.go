package pipeline

import (
	"github.com/ssemakov/lkmcu/internal/layout"
	"github.com/ssemakov/lkmcu/internal/livekit"
)

type EventType string

type SystemEvent interface { isSystemEvent() }

type (
	RoomChangedEvent struct{
		LkEvents []livekit.Event // can contain multiple coalesced eventss
	}
	LayoutChangedEvent struct{ Layout layout.LayoutView }
)

func (RoomChangedEvent) isSystemEvent() {}
func (LayoutChangedEvent) isSystemEvent() {}
