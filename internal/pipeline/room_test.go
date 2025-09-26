package pipeline

import (
	"fmt"
	"testing"
)

func TestRoomSnapshot_UpsertAndDeleteParticipant(t *testing.T) {
    rs := NewRoomSnapshot()
    pid := "pid1"
    identity := "user1"

    // Upsert participant
    rs.UpsertParticipant(pid, identity)
    if got, ok := rs.Participants[pid]; !ok {
        t.Fatalf("participant not found after upsert")
    } else if got.Identity != identity {
        t.Errorf("expected identity %s, got %s", identity, got.Identity)
    }

    // Upsert same participant with new identity
    newIdentity := "user1-renamed"
    rs.UpsertParticipant(pid, newIdentity)
    if got := rs.Participants[pid]; got.Identity != newIdentity {
        t.Errorf("expected updated identity %s, got %s", newIdentity, got.Identity)
    }

    // Delete participant
    rs.DeleteParticipant(pid)
    if _, ok := rs.Participants[pid]; ok {
        t.Errorf("participant not deleted")
    }
}

func TestRoomSnapshot_UpsertAndDeleteTrack(t *testing.T) {
    rs := NewRoomSnapshot()
    pid := "pid2"
    tid := "track1"
    kind := TrackAudio

    rs.UpsertParticipant(pid, "user2")

    track, err := rs.UpsertTrack(pid, tid, kind)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if track == nil {
        t.Fatalf("track is nil after upsert")
    }
    if track.ID != tid || track.Kind != kind {
        t.Errorf("unexpected track fields: %+v", track)
    }

    newKind := TrackVideo
    track2, err := rs.UpsertTrack(pid, tid, newKind)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if track2.Kind != newKind {
        t.Errorf("expected kind %v, got %v", newKind, track2.Kind)
    }

    rs.DeleteTrack(pid, tid)
    if p, ok := rs.Participants[pid]; ok {
        if _, ok2 := p.PublishedTracks[tid]; ok2 {
            t.Errorf("track not deleted")
        }
    } else {
        t.Fatalf("participant missing after deleting track")
    }
}

func TestRoomSnapshot_UpsertTrack_Errors(t *testing.T) {
    rs := NewRoomSnapshot()
    pid := "missing"
    tid := "trackX"
    kind := TrackAudio

    track, err := rs.UpsertTrack(pid, tid, kind)
    if err == nil {
        t.Errorf("expected error for missing participant")
    }
    if track != nil {
        t.Errorf("expected nil track for missing participant")
    }
}

func TestRoomSnapshot_DeleteTrack_Nonexistent(t *testing.T) {
    rs := NewRoomSnapshot()
    pid := "pid3"
    tid := "trackY"

    // Should not panic or error if deleting track from missing participant
    rs.DeleteTrack(pid, tid)

    // Add participant, but not the track
    rs.UpsertParticipant(pid, "user3")
    rs.DeleteTrack(pid, tid)
}

func TestRoomSnapshot_MultipleParticipantsAndTracks(t *testing.T) {
    rs := NewRoomSnapshot()
    // Add multiple participants and tracks
    for i := 0; i < 3; i++ {
        pid := fmt.Sprintf("p%d", i)
        rs.UpsertParticipant(pid, fmt.Sprintf("user%d", i))
        for j := 0; j < 2; j++ {
            tid := fmt.Sprintf("t%d", j)
            kind := TrackAudio
            if j%2 == 1 {
                kind = TrackVideo
            }
            _, err := rs.UpsertTrack(pid, tid, kind)
            if err != nil {
                t.Fatalf("unexpected error: %v", err)
            }
        }
    }
    // Check counts
    if len(rs.Participants) != 3 {
        t.Errorf("expected 3 participants, got %d", len(rs.Participants))
    }
    for _, p := range rs.Participants {
        if len(p.PublishedTracks) != 2 {
            t.Errorf("expected 2 tracks per participant, got %d", len(p.PublishedTracks))
        }
    }
    // Delete all tracks and participants
    for pid := range rs.Participants {
        for tid := range rs.Participants[pid].PublishedTracks {
            rs.DeleteTrack(pid, tid)
        }
        rs.DeleteParticipant(pid)
    }
    if len(rs.Participants) != 0 {
        t.Errorf("expected 0 participants after deletion, got %d", len(rs.Participants))
    }
}
