package pipeline

import (
	"fmt"
	"strings"
)

type TrackKind string

const (
	TrackAudio       TrackKind = "audio"
	TrackVideo       TrackKind = "video"
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
	Tracks       map[string]*TrackInfo       // track ID -> info
	Participants map[string]*ParticipantInfo // participant ID -> info
	CurrentSubs  map[string]DecisionAction   // track ID -> current decision
	Version      uint64
}

func NewRoomSnapshot() *RoomSnapshot {
	return &RoomSnapshot{
		Tracks:       make(map[string]*TrackInfo),
		Participants: make(map[string]*ParticipantInfo),
		CurrentSubs:  make(map[string]DecisionAction),
		Version:      0,
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

func (rs *RoomSnapshot) UpsertParticipant(pid string, identity string) *ParticipantInfo {
	if p, ok := rs.Participants[pid]; ok {
		p.Identity = identity
		return p
	} else {
		p := &ParticipantInfo{ID: pid, Identity: identity}
		rs.Participants[pid] = p
		return p
	}
}

func (rs *RoomSnapshot) DeleteParticipant(pid string) {
	delete(rs.Participants, pid)
}

func (rs *RoomSnapshot) UpsertTrack(pid string, tid string, kind TrackKind) (*TrackInfo, error) {
	p, ok := rs.Participants[pid]
	if !ok {
		return nil, fmt.Errorf("UpsertTrack for unknown participant %s", pid)
	}

	if t, ok := p.PublishedTracks[tid]; ok {
		t.Kind = kind
		return t, nil
	} else {
		t := &TrackInfo{ID: tid, Kind: kind, PID: pid}
		if p.PublishedTracks == nil {
			p.PublishedTracks = make(map[string]*TrackInfo)
		}
		p.PublishedTracks[tid] = t
		rs.Tracks[tid] = t
		return t, nil
	}
}

func (rs *RoomSnapshot) DeleteTrack(pid string, tid string) error {
	p, ok := rs.Participants[pid]
	if !ok {
		return fmt.Errorf("DeleteTrack for unknown participant %s", pid)
	}
	delete(p.PublishedTracks, tid)
	delete(rs.Tracks, tid)

	return nil
}
