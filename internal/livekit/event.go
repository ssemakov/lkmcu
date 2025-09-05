package livekit

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
