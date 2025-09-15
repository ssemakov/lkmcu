package pipeline

import (
	"fmt"
	"strings"
)

type TrackKind string
const (
	TrackAudio      TrackKind = "audio"
	TrackVideo      TrackKind = "video"
	TrackScreenShare TrackKind = "screenshare"
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
	Tracks 	     map[string]*TrackInfo       // track ID -> info
	Participants map[string]*ParticipantInfo // participant ID -> info
	CurrentSubs  map[string]DecisionAction   // track ID -> current decision
	Version      uint64
}

func NewRoomSnapshot() *RoomSnapshot {
	return &RoomSnapshot{
		Tracks: make(map[string]*TrackInfo),
		Participants: make(map[string]*ParticipantInfo),
		CurrentSubs: make(map[string]DecisionAction),
		Version: 0,
	}
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

func (rs *RoomSnapshot) UpsertParticipant(PID string, Identity string) *ParticipantInfo {
	if p, ok := rs.Participants[PID]; ok {
		p.Identity = Identity
		return p
	} else {
		p := &ParticipantInfo{ ID: PID, Identity: Identity }
		rs.Participants[PID] = p
		return p
	}
}

func (rs *RoomSnapshot) DeleteParticipant(PID string) {
	delete(rs.Participants, PID)
}

func (rs *RoomSnapshot) UpsertTrack(PID string, TID string, Kind TrackKind) (*TrackInfo, error) {
	p, ok := rs.Participants[PID]
	if !ok {
		return nil, fmt.Errorf("UpsertTrack for unknown participant %s", PID)
	}

	if t, ok := p.PublishedTracks[TID]; ok {
		t.Kind = Kind
		return t, nil
	} else {
		t := &TrackInfo{ ID: TID, Kind: Kind, PID: PID }
		if p.PublishedTracks == nil {
			p.PublishedTracks = make(map[string]*TrackInfo)
		}
		p.PublishedTracks[TID] = t
		rs.Tracks[TID] = t
		return t, nil
	}
}

func (rs *RoomSnapshot) DeleteTrack(PID string, TID string) error {
	p, ok := rs.Participants[PID]
	if !ok {
		return fmt.Errorf("DeleteTrack for unknown participant %s", PID)
	}
	delete(p.PublishedTracks, TID)
	delete(rs.Tracks, TID)

	return nil
}
