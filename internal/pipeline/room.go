package pipeline

import (
	"fmt"
	"strings"
)

// Describes a track in the room
type TrackInfo struct {
	ID   string
	Kind TrackKind
	PID  string // participant ID
}

type ParticipantInfo struct {
	ID              string
	Identity        string
	PublishedTracks map[string]*TrackInfo // track ID -> info
}

// Current snapshot of the room as we see it
type RoomSnapshot struct {
	Participants map[string]*ParticipantInfo // participant ID -> info
	CurrentSubs  map[string]DecisionAction   // track ID -> current decision
	Version      uint64
}

// logging Format implementations
func (t *TrackInfo) Format() string {
	if t == nil {
		return "<nil>"
	}
	return fmt.Sprintf("TrackInfo{ID:%s Kind:%s PID:%s}", t.ID, t.Kind, t.PID)
}

func (p *ParticipantInfo) Format() string {
	if p == nil {
		return "<nil>"
	}
	var tracks []string
	for tid, t := range p.PublishedTracks {
		tracks = append(tracks, fmt.Sprintf("%s:%s", tid, t.Format()))
	}
	return fmt.Sprintf("ParticipantInfo{ID:%s Identity:%s PublishedTracks:{%s}}", p.ID, p.Identity, strings.Join(tracks, " "))
}

func (rs *RoomSnapshot) Format() string {
	if rs == nil {
		return "<nil>"
	}
	var participants []string
	for pid, p := range rs.Participants {
		participants = append(participants, fmt.Sprintf("%s:%s", pid, p.Format()))
	}
	return fmt.Sprintf("RoomSnapshot{Participants:{%s} CurrentSubs:%+v Version:%d}", strings.Join(participants, " "), rs.CurrentSubs, rs.Version)
}
