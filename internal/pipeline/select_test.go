package pipeline

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/ssemakov/lkmcu/internal/layout"
	"github.com/ssemakov/lkmcu/internal/livekit"
)

func TestDecide(t *testing.T) {
    tests := []struct {
        name     string
        policy   *Policy
        layout   *layout.LayoutView
        snapshot *RoomSnapshot
        want     []Decision
    }{
        {
            name: "empty snapshot returns empty decisions",
            policy: &Policy{
                IncludeKinds: []TrackKind{TrackVideo, TrackAudio},
            },
            layout:   &layout.LayoutView{StageTracks: map[string]string{}},
            snapshot: &RoomSnapshot{Tracks: map[string]*TrackInfo{}, Participants: map[string]*ParticipantInfo{}},
            want:     []Decision{},
        },
				{
						name: "track with missing info",
						policy: &Policy{},
						layout: &layout.LayoutView{StageTracks: map[string]string{"track1": "p1"}},
						snapshot: &RoomSnapshot{
							Tracks: map[string]*TrackInfo{
								"track1": nil,
							},
							Participants: map[string]*ParticipantInfo{
								"p1": {ID: "p1"},
							},
						},
						want: []Decision{}, // should skip track with missing info
				},
				{ 	name: "track with missing participant",
						policy: &Policy{},
						layout: &layout.LayoutView{StageTracks: map[string]string{"track1": "p1"}},
						snapshot: &RoomSnapshot{
							Tracks: map[string]*TrackInfo{
								"track1": {PID: "p1", Kind: TrackVideo},
							},
							Participants: map[string]*ParticipantInfo{},
						},
						want: []Decision{}, // should skip track with missing participant
				},
        {
            name: "excluded track by policy",
            policy: &Policy{
                IncludeKinds: []TrackKind{TrackAudio},
                ExcludeKinds: []TrackKind{TrackVideo},
            },
            layout: &layout.LayoutView{StageTracks: map[string]string{"track1": "p1", "track2": "p1"}},
            snapshot: &RoomSnapshot{
                Tracks: map[string]*TrackInfo{
                    "track1": {PID: "p1", Kind: TrackVideo},
										"track2": {PID: "p1", Kind: TrackAudio},
                },
                Participants: map[string]*ParticipantInfo{
                    "p1": {ID: "p1"},
                },
            },
            want: []Decision{
                {TrackID: "track1", Action: Paused, Reason: "excluded by policy"},
								{TrackID: "track2", Action: Audio, Reason: "audio on stage"},
            },
        },
				{
					name: "only included tracks by policy",
					policy: &Policy{
						IncludeKinds: []TrackKind{TrackVideo},
					},
					layout: &layout.LayoutView{StageTracks: map[string]string{"track1": "p1", "track2": "p1"}},
					snapshot: &RoomSnapshot{
						Tracks: map[string]*TrackInfo{
							"track1": {PID: "p1", Kind: TrackVideo},
							"track2": {PID: "p1", Kind: TrackAudio},
						},
						Participants: map[string]*ParticipantInfo{
							"p1": {ID: "p1"},
						},
					},
					want: []Decision{
						{TrackID: "track1", Action: VideoFull, Reason: "video on stage"},
						{TrackID: "track2", Action: Paused, Reason: "excluded by policy"},
					},
				},
				{
            name: "track not on stage",
            policy: &Policy{
                IncludeKinds: []TrackKind{TrackVideo, TrackAudio},
            },
            layout: &layout.LayoutView{StageTracks: map[string]string{"track2": "p1"}},
            snapshot: &RoomSnapshot{
                Tracks: map[string]*TrackInfo{
                    "track1": {PID: "p1", Kind: TrackVideo},
                },
                Participants: map[string]*ParticipantInfo{
                    "p1": {ID: "p1"},
                },
            },
            want: []Decision{
                {TrackID: "track1", Action: Paused, Reason: "not on stage"},
            },
        },
        {
            name: "video track on stage",
            policy: &Policy{
                IncludeKinds: []TrackKind{TrackVideo, TrackAudio},
            },
            layout: &layout.LayoutView{StageTracks: map[string]string{"track1": "p1"}},
            snapshot: &RoomSnapshot{
                Tracks: map[string]*TrackInfo{
                    "track1": {PID: "p1", Kind: TrackVideo},
                },
                Participants: map[string]*ParticipantInfo{
                    "p1": {ID: "p1"},
                },
            },
            want: []Decision{
                {TrackID: "track1", Action: VideoFull, Reason: "video on stage"},
            },
        },
        {
            name: "audio track on stage",
            policy: &Policy{
                IncludeKinds: []TrackKind{TrackVideo, TrackAudio},
            },
            layout: &layout.LayoutView{StageTracks: map[string]string{"track1": "p1"}},
            snapshot: &RoomSnapshot{
                Tracks: map[string]*TrackInfo{
                    "track1": {PID: "p1", Kind: TrackAudio},
                },
                Participants: map[string]*ParticipantInfo{
                    "p1": {ID: "p1"},
                },
            },
            want: []Decision{
                {TrackID: "track1", Action: Audio, Reason: "audio on stage"},
            },
        },
        {
            name: "multiple tracks with different states",
            policy: &Policy{
                IncludeKinds: []TrackKind{TrackVideo, TrackAudio},
            },
            layout: &layout.LayoutView{StageTracks: map[string]string{
                "video1": "p1",
                "audio1": "p1",
            }},
            snapshot: &RoomSnapshot{
                Tracks: map[string]*TrackInfo{
                    "video1": {PID: "p1", Kind: TrackVideo},
                    "audio1": {PID: "p1", Kind: TrackAudio},
                    "video2": {PID: "p2", Kind: TrackVideo},
                },
                Participants: map[string]*ParticipantInfo{
                    "p1": {ID: "p1"},
                    "p2": {ID: "p2"},
                },
            },
            want: []Decision{
                {TrackID: "video1", Action: VideoFull, Reason: "video on stage"},
                {TrackID: "audio1", Action: Audio, Reason: "audio on stage"},
                {TrackID: "video2", Action: Paused, Reason: "not on stage"},
            },
        },
    }

		sorter := cmpopts.SortSlices(func(a, b Decision) bool {
			if a.TrackID != b.TrackID {
				return a.TrackID < b.TrackID
			}
			if a.Action != b.Action {
				return a.Action < b.Action
			}
			return a.Reason < b.Reason
		})

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := Decide(tt.policy, tt.layout, tt.snapshot)
            if diff := cmp.Diff(tt.want, got, sorter); diff != "" {
                t.Errorf("Decide() mismatch (-want +got):\n%s", diff)
            }
        })
    }
}

func TestDiff(t *testing.T) {
    tests := []struct {
        name string
        cur  []*livekit.Subscription
        decs []Decision
        want []Operations
    }{
        {
            name: "empty inputs return empty operations",
            cur:  []*livekit.Subscription{},
            decs: []Decision{},
            want: []Operations{},
        },
        {
            name: "subscribe to new track",
            cur:  []*livekit.Subscription{},
            decs: []Decision{
                {TrackID: "track1", Action: VideoFull, Reason: "video on stage"},
            },
            want: []Operations{
                &SubscribeOp{TrackID: "track1", Action: VideoFull, Reason: "video on stage"},
            },
        },
        {
            name: "unsubscribe from track",
            cur: []*livekit.Subscription{
                {TID: "track1"},
            },
            decs: []Decision{
                {TrackID: "track1", Action: Paused, Reason: "not on stage"},
            },
            want: []Operations{
                &UnsubscribeOp{TrackID: "track1", Reason: "not on stage"},
            },
        },
        {
            name: "no change for already subscribed track",
            cur: []*livekit.Subscription{
                {TID: "track1"},
            },
            decs: []Decision{
                {TrackID: "track1", Action: VideoFull, Reason: "video on stage"},
            },
            want: []Operations{},
        },
        {
            name: "mix of subscribe and no change",
            cur: []*livekit.Subscription{
                {TID: "track1"},
            },
            decs: []Decision{
                {TrackID: "track1", Action: VideoFull, Reason: "video on stage"},
                {TrackID: "track2", Action: Audio, Reason: "audio on stage"},
            },
            want: []Operations{
                &SubscribeOp{TrackID: "track2", Action: Audio, Reason: "audio on stage"},
            },
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := Diff(tt.cur, tt.decs)
            if diff := cmp.Diff(tt.want, got, cmp.AllowUnexported(SubscribeOp{}, UnsubscribeOp{})); diff != "" {
                t.Errorf("Diff() mismatch (-want +got):\n%s", diff)
            }
        })
    }
}

