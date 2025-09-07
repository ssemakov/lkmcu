package livekit

import (
	"fmt"
	"reflect"
)

// LiveKit normalizes events as Event interface
type Event interface { isLiveKitEvent() }

type (
	ParticipantJoined 	struct{ PID, Identity string }
	ParticipantLeft   	struct{ PID string }
	TrackPublished   	struct{ PID, TID, Kind string }
	TrackUnpublished 	struct{ PID, TID string }
	TrackSubscribed   	struct{ PID, TID, Kind string; Track interface{} }
	TrackUnsubscribed 	struct{ PID, TID, Kind string; Track interface{} }
	RoomDisconnected 	struct{ Reason string }
	RoomReconnected   	struct{}
)

func (ParticipantJoined) isLiveKitEvent()   {}
func (ParticipantLeft) isLiveKitEvent()     {}
func (TrackPublished) isLiveKitEvent()      {}
func (TrackUnpublished) isLiveKitEvent()    {}
func (TrackSubscribed) isLiveKitEvent()     {}
func (TrackUnsubscribed) isLiveKitEvent()   {}
func (RoomDisconnected) isLiveKitEvent()   {}
func (RoomReconnected) isLiveKitEvent()      {}

// formatEvent is a generic function to format any struct with field names
func formatEvent[T Event](event T) string {
	typeName := reflect.TypeOf(event).Name()
	return fmt.Sprintf("%s(%+v)", typeName, event)
}

// Format methods for all event types using the generic function
func (e ParticipantJoined) Format() string   { return formatEvent(e) }
func (e ParticipantLeft) Format() string     { return formatEvent(e) }
func (e TrackPublished) Format() string      { return formatEvent(e) }
func (e TrackUnpublished) Format() string    { return formatEvent(e) }
func (e TrackSubscribed) Format() string     { return formatEvent(e) }
func (e TrackUnsubscribed) Format() string   { return formatEvent(e) }
func (e RoomDisconnected) Format() string    { return formatEvent(e) }
func (e RoomReconnected) Format() string     { return formatEvent(e) }
