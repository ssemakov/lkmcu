package layout

import (
	"encoding/json"
	"io"
	"net/http"
)

// Basic view of the layout, which tracks are on stage, will be extended later
type LayoutView struct {
	StageTracks       map[string]string   // Track SID -> participant SID, can have multiple tracks from same participant
	StageParticipants map[string][]string // participant Identity -> allowed kinds ("audio","video","screenshare")
	CurentStage       *Stage
}

// Stage Slot
// ID is optional, mainly for client-side convenience
// Rect if is not defined, the full stage area is used, if more than one slot has no rect, they are stacked
//
//	Z is used to define stacking order, higher Z is on top
//	if TrackKind is "audio" then Rect is ignored
//
// TrackID is optional if Identity and TrackKind are set
// Identity is optional if TrackID is set. The Participant must have a published track of the given kind
// TrackKind is optional, if TrackID is set. The kind must match the published track kind
type Slot struct {
	// Id        int    `json:"id,omitempty"`
	Rect      *Rect  `json:"rect"`
	TrackID   string `json:"trackId,omitempty"`
	Identity  string `json:"identity,omitempty"`  // participant identity
	TrackKind string `json:"trackKind,omitempty"` // "audio", "video", "screen"
}

type Rect struct {
	X float32 `json:"x"`
	Y float32 `json:"y"`
	W float32 `json:"w"`
	H float32 `json:"h"`
	Z int     `json:"z"`
}

type Stage struct {
	Slots []Slot `json:"slots"`
}

func DefaultStage() *Stage {
	return &Stage{Slots: []Slot{}}
}

func DefaultLayoutView() *LayoutView {
	return &LayoutView{
		StageTracks:       make(map[string]string),
		StageParticipants: make(map[string][]string),
		CurentStage:       DefaultStage(),
	}
}

func NewLayoutView(initialStage string) (*LayoutView, error) {
	var (
		s   *Stage
		err error
	)

	if initialStage != "" {
		s, err = parseStage(initialStage)
		if err != nil {
			return nil, err
		}
	} else {
		s = DefaultStage()
	}

	return &LayoutView{
		StageTracks:       make(map[string]string),
		CurentStage:       s,
		StageParticipants: StageParticipantsFromStageLayout(s),
	}, nil
}

func parseStage(jsonStr string) (*Stage, error) {
	var s Stage

	if err := json.Unmarshal([]byte(jsonStr), &s); err != nil {
		return nil, err
	}

	return &s, nil
}

// StageParticipantsFromStageLayout extracts participant information from a Stage layout.
// It returns a map where keys are participant identities and values are arrays of track kinds
// (e.g., "audio", "video", "screenshare") that each participant has in the stage layout.
// This is used to determine which participants and which of their track types should be included
// in the composed output.
func StageParticipantsFromStageLayout(s *Stage) map[string][]string {
	participants := make(map[string][]string)

	for _, slot := range s.Slots {
		if slot.Identity != "" {
			participants[slot.Identity] = append(participants[slot.Identity], slot.TrackKind)
		}
	}

	return participants
}

func HTTPHandler(stage *Stage, w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(stage)
	case http.MethodPost:
		body, _ := io.ReadAll(r.Body)
		var s Stage
		if err := json.Unmarshal(body, &s); err != nil || len(s.Slots) == 0 {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("invalid layout"))
			return
		}
		*stage = s
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
