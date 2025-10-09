package pipeline

import (
	"github.com/ssemakov/lkmcu/internal/layout"
	"github.com/ssemakov/lkmcu/internal/livekit"
)

type DecisionAction string

const (
	VideoFull DecisionAction = "video-full"
	VideoLow  DecisionAction = "video-low"
	Audio     DecisionAction = "audio"
	Paused    DecisionAction = "paused"
)

type Decision struct {
	TrackID string
	Action  DecisionAction
	Reason  string
}

type Operations interface {
	isOperation()
	exec(lk *livekit.Client) error
}

type SubscribeOp struct {
	TrackID string
	Action  DecisionAction
	Reason  string
}
type UnsubscribeOp struct {
	TrackID string
	Reason  string
}

type UpdateOp struct {
	TrackID string
	Action  DecisionAction
	Reason  string
}

func (o *SubscribeOp) isOperation()   {}
func (o *UnsubscribeOp) isOperation() {}
func (o *UpdateOp) isOperation()      {}

func (o *SubscribeOp) String() string {
	return "SubscribeOp{ TrackID: " + o.TrackID + ", Action: " + string(o.Action) + ", Reason: " + o.Reason + " }"
}

func (o *UnsubscribeOp) String() string {
	return "UnsubscribeOp{ TrackID: " + o.TrackID + ", Reason: " + o.Reason + " }"
}

func (o *UpdateOp) String() string {
	return "UpdateOp{ TrackID: " + o.TrackID + ", Action: " + string(o.Action) + ", Reason: " + o.Reason + " }"
}

func (o *SubscribeOp) exec(lk *livekit.Client) error {
	// return lk.Subscribe(o.TrackID, o.Action)
	return nil
}

func (o *UnsubscribeOp) exec(lk *livekit.Client) error {
	// return lk.Unsubscribe(o.TrackID)
	return nil
}

func (o *UpdateOp) exec(lk *livekit.Client) error {
	// return lk.UpdateSubscription(o.TrackID, o.Action)
	return nil
}

func Decide(policy Policy, layout *layout.LayoutView, snapshot *RoomSnapshot) []Decision {
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

		// track on stage? // either check CurrenStage slots or update StageTracks on layout change
		_, onStage := layout.StageTracks[tid]                  // check if the track is on stage
		_, onStage := layout.StageParticipants[pInfo.Identity] // check if the participant is on stage
		for _, tk := range layout.StageParticipants[pInfo.Identity] {
			if tk == tInfo.Kind {
				onStage = true
				break
			}
		}

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
	ops := make([]Operations, 0)

	// Subscribe/Update operations
	for _, d := range decs {
		switch d.Action {
		case VideoFull, VideoLow, Audio:
			found := false
			for _, s := range cur {
				if s.TID == d.TrackID {
					found = true
					break // no op for now,  already subscribed
				}
			}

			if !found {
				// TODO change the quality layer and update if needed
				ops = append(ops, &SubscribeOp{TrackID: d.TrackID, Action: d.Action, Reason: d.Reason})
			}
		case Paused:
			for _, s := range cur {
				if s.TID == d.TrackID {
					return []Operations{&UnsubscribeOp{TrackID: d.TrackID, Reason: d.Reason}}
				}
			}
		}
	}

	// Unsubscribe operations
	want := make(map[string]DecisionAction, len(decs))
	for _, d := range decs {
		want[d.TrackID] = d.Action
	}
	for _, s := range cur {
		if _, ok := want[s.TID]; !ok {
			ops = append(ops, &UnsubscribeOp{TrackID: s.TID, Reason: "not in decisions / participant left"})
		}
	}

	return ops
}
