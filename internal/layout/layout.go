package layout

import (
	"encoding/json"
	"io"
	"net/http"
)

// Basic view of the layout, which tracks are on stage, will be extended later
type LayoutView struct {
	StageTracks map[string]string // track ID -> participant ID, can have both audio and video tracks
}

type Slot struct {
	Id	    int		  `json:"id,omitempty"`
	TrackID string  `json:"trackId,omitempty"`
	Rect    Rect    `json:"rect"`
	Effects []string `json:"effects,omitempty"`
}

type Rect struct {
	X float32 `json:"x"`
	Y float32 `json:"y"`
	W float32 `json:"w"`
	H float32 `json:"h"`
	Z int     `json:"z"`
}

type Stage struct {
	Slots []Rect `json:"slots"`
}

func Default() Stage {
	return Stage{Slots: []Rect{
		{X: 0, Y: 0, W: 0.5, H: 0.5, Z: 0},
		{X: 0.5, Y: 0, W: 0.5, H: 0.5, Z: 0},
		{X: 0, Y: 0.5, W: 0.5, H: 0.5, Z: 0},
		{X: 0.5, Y: 0.5, W: 0.5, H: 0.5, Z: 0},
	}}
}

func NewLayoutView() *LayoutView {
	return &LayoutView{
		StageTracks: make(map[string]string),
	}
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
