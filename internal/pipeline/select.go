package pipeline

import (
	"slices"

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
	PID     string
	Action  DecisionAction
	Reason  string
}

type Operations interface {
	isOperation()
	Exec(lk livekit.Client) error
}

type SubscribeOp struct {
	TrackID string
	PID     string
	Action  DecisionAction
	Reason  string
}
type UnsubscribeOp struct {
	TrackID string
	PID     string
	Reason  string
}

type UpdateOp struct {
	TrackID string
	PID     string
	Action  DecisionAction
	Reason  string
}

func (o *SubscribeOp) isOperation()   {}
func (o *UnsubscribeOp) isOperation() {}
func (o *UpdateOp) isOperation()      {}

func (o *SubscribeOp) String() string {
	return "SubscribeOp{ TrackID: " + o.TrackID + ", PID: " + o.PID + ", Action: " + string(o.Action) + ", Reason: " + o.Reason + " }"
}

func (o *UnsubscribeOp) String() string {
	return "UnsubscribeOp{ TrackID: " + o.TrackID + ", PID: " + o.PID + ", Reason: " + o.Reason + " }"
}

func (o *UpdateOp) String() string {
	return "UpdateOp{ TrackID: " + o.TrackID + ", PID: " + o.PID + ", Action: " + string(o.Action) + ", Reason: " + o.Reason + " }"
}

func (o *SubscribeOp) Exec(lk livekit.Client) error {
	return lk.Subscribe(o.PID, o.TrackID, true)
}

func (o *UnsubscribeOp) Exec(lk livekit.Client) error {
	return lk.Subscribe(o.PID, o.TrackID, false)
}

func (o *UpdateOp) Exec(lk livekit.Client) error {
	// return lk.UpdateSubscription(o.TrackID, o.Action)
	return nil
}

// Decide computes desired subscription actions for all tracks given:
// - policy: inclusion/exclusion of track kinds
// - layout: on-stage participants and tracks
// - snapshot: current room state
//
// Join model:
// - For each track in snapshot.Tracks (key = Track SID):
//  1. Verify track and its owner participant exist.
//     - Participant is looked up via TrackInfo.ParticipantSID (Participant SID) in snapshot.Participants.
//  2. Determine if track is “on stage”:
//     - Directly: layout.StageTracks[trackKey] presence means on stage.
//     - By identity: take participant.Identity, then check if it appears in
//     layout.StageParticipants with a kind matching TrackInfo.Kind.
//  3. Apply policy include/exclude rules. If excluded or not on stage → Paused.
//  4. Otherwise:
//     - video/screenshare → VideoFull
//     - audio → Audio
//
// Output:
//   - A slice of Decision values, one per considered track, where Decision.TrackID equals
//     the map key (trackKey) used in snapshot.Tracks.
func Decide(policy Policy, layout *layout.LayoutView, snapshot *RoomSnapshot) []Decision {
	decs := []Decision{}

	for tid, tInfo := range snapshot.Tracks {
		// track presence
		if tInfo == nil {
			continue
		}

		// participant presence by SID (ParticipantSID)
		pid := tInfo.ParticipantSID
		pInfo, ok := snapshot.Participants[pid]
		if !ok || pInfo == nil {
			continue
		}

		// policy
		included := policy.IsIncluded(tInfo.Kind)
		excluded := policy.IsExcluded(tInfo.Kind)

		if !included || excluded {
			decs = append(decs, Decision{TrackID: tid, PID: pid, Action: Paused, Reason: "excluded by policy"})
			continue
		}

		// on-stage by Track SID presence or by participant identity + kind
		// layout.StageParticipants is keyed by participant identity
		onStage := false
		if layout.StageTracks[tid] != "" {
			onStage = true
			// or check if the participant is on stage with the given track kind
		} else if kinds, ok := layout.StageParticipants[pInfo.Identity]; ok {
			onStage = slices.Contains(kinds, tInfo.Kind.String())
		}

		if !onStage {
			decs = append(decs, Decision{TrackID: tid, PID: pid, Action: Paused, Reason: "not on stage"})
			continue
		}

		switch tInfo.Kind {
		case TrackVideo, TrackScreenShare:
			decs = append(decs, Decision{TrackID: tid, PID: pid, Action: VideoFull, Reason: "video on stage"})
		case TrackAudio:
			decs = append(decs, Decision{TrackID: tid, PID: pid, Action: Audio, Reason: "audio on stage"})
		default:
			decs = append(decs, Decision{TrackID: tid, PID: pid, Action: Paused, Reason: "unknown track kind"})
		}
	}

	return decs
}

// Diff converts desired decisions into subscription operations compared to current subscriptions.
// - cur: current LiveKit subscriptions (TID = trackKey/SID as used by Decide outputs)
// - decs: target high-level decisions from Decide
//
// Behavior:
// - For decisions with action VideoFull/VideoLow/Audio: subscribe if not already subscribed.
// - For Paused decisions: unsubscribe if currently subscribed.
// - Also unsubscribe any current subscriptions not present in decs (participant/track gone).
//
// Returns a list of operations (SubscribeOp/UnsubscribeOp/UpdateOp) to reach the target state.
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
				ops = append(ops, &SubscribeOp{TrackID: d.TrackID, PID: d.PID, Action: d.Action, Reason: d.Reason})
			}
		case Paused:
			for _, s := range cur {
				if s.TID == d.TrackID {
					return []Operations{&UnsubscribeOp{TrackID: d.TrackID, PID: d.PID, Reason: d.Reason}}
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
			ops = append(ops, &UnsubscribeOp{TrackID: s.TID, PID: s.PID, Reason: "not in decisions / participant left"})
		}
	}

	return ops
}
