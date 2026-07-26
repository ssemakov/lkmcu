package livekit

// Event interface for normalazing LiveKit events.
type Event interface{ isLiveKitEvent() }

type (
	ParticipantJoined struct{ PID, Identity string }
	ParticipantLeft   struct{ PID, Identity string }
	TrackPublished    struct{ PID, Identity, TID, Kind string }
	TrackUnpublished  struct{ PID, Identity, TID, Kind string }
	TrackSubscribed   struct {
		PID, Identity, TID, Kind string
		RemoteTrack              RemoteTrack
		Receiver                 Receiver
	}
	TrackUnsubscribed struct {
		PID, Identity, TID, Kind string
		RemoteTrack              RemoteTrack
	}
	RoomDisconnected struct{ Reason string }
	RoomReconnected  struct{}
	RoomClosed       struct{}
)

func (ParticipantJoined) isLiveKitEvent() {}
func (ParticipantLeft) isLiveKitEvent()   {}
func (TrackPublished) isLiveKitEvent()    {}
func (TrackUnpublished) isLiveKitEvent()  {}
func (TrackSubscribed) isLiveKitEvent()   {}
func (TrackUnsubscribed) isLiveKitEvent() {}
func (RoomDisconnected) isLiveKitEvent()  {}
func (RoomReconnected) isLiveKitEvent()   {}
func (RoomClosed) isLiveKitEvent()        {}

// Format methods for all event types using the generic function
