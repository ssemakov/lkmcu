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

func ParseTrackKind(kindStr string) (TrackKind, error) {
	kind := TrackKind(kindStr)
	switch kind {
	case TrackAudio, TrackVideo, TrackScreenShare:
		return kind, nil
	default:
		return "", fmt.Errorf("unknown track kind: %s", kindStr)
	}
}

func (k TrackKind) String() string {
	return string(k)
}

// TrackInfo describes a published track (LiveKit concepts).
// - TrackSID: Track SID (“TR…”), server-generated opaque ID.
// - ParticipantSID: Participant SID (“PA…”) of the track owner.
type TrackInfo struct {
	TrackSID       string    // Track SID (LiveKit TR_XXXX)
	Kind           TrackKind // audio | video | screenshare
	ParticipantSID string    // Participant SID (LiveKit PA_XXXX)
}

// ParticipantInfo mirrors LiveKit participant metadata.
// - ID: Participant SID (“PA…”), server-generated opaque ID.
// - Identity: participant identity (app-defined, stable), used for UI/layout logic.
type ParticipantInfo struct {
	ParticipantSID  string                // Participant SID (LiveKit PA_XXXX)
	Identity        string                // Participant identity (LiveKit Identity)
	PublishedTracks map[string]*TrackInfo // track ID -> info
}

// RoomSnapshot is the current view of the room for selection logic.
// Map key semantics:
// - Tracks: keyed by trackKey (prefer Track SID). Decide() returns trackKey in its output.
// - Participants: keyed by Participant SID (matches TrackInfo.ParticipantSID).
// - CurrentSubs: current decisions, keyed by trackKey.
type RoomSnapshot struct {
	Tracks       map[string]*TrackInfo       // Track SID -> info
	Participants map[string]*ParticipantInfo // Participant SID (matches TrackInfo.ParticipantSID) -> info
	CurrentSubs  map[string]DecisionAction   // Track SID -> current decision
}

func NewRoomSnapshot() *RoomSnapshot {
	return &RoomSnapshot{
		Tracks:       make(map[string]*TrackInfo),
		Participants: make(map[string]*ParticipantInfo),
		CurrentSubs:  make(map[string]DecisionAction),
	}
}

// logging Format implementations
func (t *TrackInfo) Format() string {
	if t == nil {
		return "<nil>"
	}
	return fmt.Sprintf("TrackInfo{SID:%s Kind:%s ParticipantSID:%s}", t.TrackSID, t.Kind, t.ParticipantSID)
}

func (p *ParticipantInfo) Format() string {
	if p == nil {
		return "<nil>"
	}
	var tracks []string
	for tid, t := range p.PublishedTracks {
		tracks = append(tracks, fmt.Sprintf("%s:%s", tid, t.Format()))
	}
	return fmt.Sprintf("ParticipantInfo{SID:%s Identity:%s PublishedTracks:{%s}}", p.ParticipantSID, p.Identity, strings.Join(tracks, " "))
}

func (rs *RoomSnapshot) Format() string {
	if rs == nil {
		return "<nil>"
	}
	var participants []string
	for pid, p := range rs.Participants {
		participants = append(participants, fmt.Sprintf("%s:%s", pid, p.Format()))
	}

	var tracks []string
	for tid, t := range rs.Tracks {
		tracks = append(tracks, fmt.Sprintf("%s:%s", tid, t.Format()))
	}

	return fmt.Sprintf("RoomSnapshot{Participants:{%s} Tracks:{%s} CurrentSubs:%+v}",
		strings.Join(participants, " "), strings.Join(tracks, " "), rs.CurrentSubs)
}

func (rs *RoomSnapshot) UpsertParticipant(pid string, identity string) *ParticipantInfo {
	if p, ok := rs.Participants[pid]; ok {
		p.Identity = identity
		return p
	} else {
		p := &ParticipantInfo{ParticipantSID: pid, Identity: identity}
		rs.Participants[pid] = p
		return p
	}
}

func (rs *RoomSnapshot) DeleteParticipant(pid string) {
	delete(rs.Participants, pid)
}

func (rs *RoomSnapshot) DeleteParticipantAndTracks(pid string) []string {
	p, ok := rs.Participants[pid]
	if !ok {
		return nil
	}

	var tids []string
	for tid := range p.PublishedTracks {
		tids = append(tids, tid)
		delete(rs.Tracks, tid)
	}
	delete(rs.Participants, pid)
	return tids
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
		t := &TrackInfo{TrackSID: tid, Kind: kind, ParticipantSID: pid}
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

// Adds or updates the subscription decision for a track in the snapshot.
func (rs *RoomSnapshot) UpsertSubCache(tid string, action DecisionAction) {
	rs.CurrentSubs[tid] = action
}

// RemoveSubCache removes the subscription decision for a track from the snapshot.
func (rs *RoomSnapshot) RemoveSubCache(tid string) {
	delete(rs.CurrentSubs, tid)
}

// Return all participant tracks to which we are subscribed
func (rs *RoomSnapshot) ParticipantTracks(pid string) ([]string, error) {
	p, ok := rs.Participants[pid]
	if !ok {
		return nil, fmt.Errorf("ParticipantTracks for unknown participant %s", pid)
	}
	tracks := p.PublishedTracks
	subs := make([]string, 0, len(tracks))

	for tid := range tracks {
		if rs.CurrentSubs[tid] != Paused {
			subs = append(subs, tid)
		}
	}

	return subs, nil
}
