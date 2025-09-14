package pipeline

type TrackKind string
const (
	TrackAudio      TrackKind = "audio"
	TrackVideo      TrackKind = "video"
	TrackScreenShare TrackKind = "screenshare"
)

// Subscription policy, allows to include/exclude certain kinds of tracks
// and limit the number of video decoders
// Should be set by the outside configuration
type Policy struct {
	IncludeKinds []TrackKind
	ExcludeKinds []TrackKind
	MaxVideoDecoders int
}


