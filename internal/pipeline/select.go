package pipeline

import (
	"github.com/ssemakov/lkmcu/internal/layout"
	"github.com/ssemakov/lkmcu/internal/livekit"
)

type DecisionAction string
const (
	VideoFull DecisionAction = "video-full"
	VideoLow  DecisionAction = "video-low"
	Audio DecisionAction = "audio"
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

	desc := []Decision{}

	for tid, tInfo := range snapshot.Tracks {
		// track presence
		if tInfo == nil {
			continue
		}

		// participant presence
		pInfo, ok := snapshot.Participants[tInfo.PID]
		if !ok || pInfo == nil {
			continue
		}

		// policy
		included := policy.IsIncluded(tInfo.Kind)
		excluded := policy.IsExcluded(tInfo.Kind)

		if !included || excluded {
			desc = append(desc, Decision{TrackID: tid, Action: Paused, Reason: "excluded by policy"})
			continue
		}

		// track on stage?
		_, onStage := layout.StageTracks[tid]
		if !onStage {
			desc = append(desc, Decision{TrackID: tid, Action: Paused, Reason: "not on stage"})
			continue
		}

		switch tInfo.Kind {
			case TrackVideo, TrackScreenShare:
				desc = append(desc, Decision{TrackID: tid, Action: VideoFull, Reason: "video on stage"})
			case TrackAudio:
				desc = append(desc, Decision{TrackID: tid, Action: Audio, Reason: "audio on stage"})
			default:
				desc = append(desc, Decision{TrackID: tid, Action: Paused, Reason: "unknown track kind"})
		}
	}

	return desc
}

func Diff(cur []*livekit.Subscription, decs []Decision) []Operations {
	// Compare current subscriptions with desired decisions and produce operations
	return []Operations{}
}
