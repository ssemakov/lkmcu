package pipeline

import (
	"github.com/ssemakov/lkmcu/internal/layout"
	"github.com/ssemakov/lkmcu/internal/livekit"
)

type DecisionAction string
const (
	VideoFull DecisionAction = "video-full"
	VideoLow  DecisionAction = "video-low"
	AudioOnly DecisionAction = "audio-only"
	Paused  DecisionAction = "paused"
)

type Decision struct {
	TrackID string
	Action  DecisionAction
	Reason  string
}

type Operations interface { isOperation() }

func Decide(policy *Policy, layout *layout.LayoutView, snapshot *RoomSnapshot) []Decision {
	/// Basic decision logic:
	// 1. Excluded? - don't subscribe
	// 2. Included and on stage? - subscribe full

	// for layout.OnStage {id, onStage := range layout.OnStage {
	// }

	return []Decision{}
}

func Diff(cur []livekit.Subscriptions, decs []Decision) []Operations {
	// Compare current subscriptions with desired decisions and produce operations
	return []Operations{}
}
