package layout

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
)

// Basic view of the layout, which tracks are on stage, will be extended later
type LayoutView struct {
	StageTracks       map[string]string   // Track SID -> participant SID, can have multiple tracks from same participant
	StageParticipants map[string][]string // participant Identity -> allowed kinds ("audio","video","screenshare")
	CurrentStage      *Stage
	mu                sync.RWMutex
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
		CurrentStage:      DefaultStage(),
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
		CurrentStage:      s,
		StageParticipants: StageParticipantsFromStageLayout(s),
	}, nil
}

// Snapshot returns a copy of the current layout view that can be safely read
// without holding locks. Maps are shallow-copied, and slice values in
// StageParticipants are copied to avoid shared mutation.
func (l *LayoutView) Snapshot() *LayoutView {
	l.mu.RLock()
	defer l.mu.RUnlock()

	st := make(map[string]string, len(l.StageTracks))
	for k, v := range l.StageTracks {
		st[k] = v
	}

	sp := make(map[string][]string, len(l.StageParticipants))
	for pid, kinds := range l.StageParticipants {
		cp := make([]string, len(kinds))
		copy(cp, kinds)
		sp[pid] = cp
	}

	return &LayoutView{
		StageTracks:       st,
		StageParticipants: sp,
		CurrentStage:      l.CurrentStage,
	}
}

// UpdateFromStage sets the current stage and recomputes StageParticipants.
// It acquires a write lock to make the update atomic.
func (l *LayoutView) UpdateFromStage(s *Stage) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.CurrentStage = s
	l.StageParticipants = StageParticipantsFromStageLayout(s)
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

func (r Rect) String() string {
	return fmt.Sprintf("Rect {X:%.2f Y:%.2f W:%.2f H:%.2f Z:%d}", r.X, r.Y, r.W, r.H, r.Z)
}

func (s Slot) String() string {
	return fmt.Sprintf("Slot {TrackID:%s Identity:%s TrackKind:%s %v}", s.TrackID, s.Identity, s.TrackKind, s.Rect)
}

func (s Stage) String() string {
	var slots []string
	for _, slot := range s.Slots {
		slots = append(slots, slot.String())
	}
	return fmt.Sprintf("Stage {Slots:[%s]}", strings.Join(slots, ", "))
}

// OnUpdateFunc is an optional callback invoked after a successful layout update.
// It receives the newly applied Stage.
type OnUpdateFunc func(newStage *Stage)

func HTTPHandler(stage *Stage, onUpdate OnUpdateFunc, w http.ResponseWriter, r *http.Request) error {
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
			return err
		}

		if onUpdate != nil {
			onUpdate(&s)
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}

	return nil
}
