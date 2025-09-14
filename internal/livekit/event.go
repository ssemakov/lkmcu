package livekit

// LiveKit normalizes events as Event interface
type Event interface { isLiveKitEvent() }

type (
	ParticipantJoined 	struct{ PID, Identity string }
	ParticipantLeft   	struct{ PID, Identity string }
	TrackPublished   	struct{ PID, Identity, TID, Kind, trackPub, participant string }
	TrackUnpublished 	struct{ PID, Identity, TID string }
	TrackSubscribed   	struct{ PID, Identity, TID, Kind string; Track interface{} }
	TrackUnsubscribed 	struct{ PID, Identity, TID, Kind string; Track interface{} }
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

// Format methods for all event types using the generic function
