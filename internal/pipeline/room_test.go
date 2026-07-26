package pipeline

import (
	"fmt"
	"slices"
	"testing"
)

func TestTrackKind_String(t *testing.T) {
	tests := []struct {
		kind TrackKind
		want string
	}{
		{TrackAudio, "audio"},
		{TrackVideo, "video"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("kind_%v", tt.kind), func(t *testing.T) {
			got := tt.kind.String()
			if got != tt.want {
				t.Errorf("TrackKind.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseTrackKind(t *testing.T) {
	tests := []struct {
		str     string
		want    TrackKind
		wantErr bool
	}{
		{"audio", TrackAudio, false},
		{"video", TrackVideo, false},
	}

	for _, tt := range tests {
		t.Run(tt.str, func(t *testing.T) {
			got, err := ParseTrackKind(tt.str)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseTrackKind(%q) error = %v, wantErr %v", tt.str, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseTrackKind(%q) = %v, want %v", tt.str, got, tt.want)
			}
		})
	}
}

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

func TestRoomSnapshot_DeleteParticipantAndTracks(t *testing.T) {
	rs := NewRoomSnapshot()
	pid := "p1"
	tids := []string{"t1", "t2"}

	rs.UpsertParticipant(pid, "user1")
	rs.UpsertTrack(pid, tids[0], TrackAudio)
	rs.UpsertTrack(pid, tids[1], TrackVideo)

	deletedTIDs := rs.DeleteParticipantAndTracks(pid)
	if _, ok := rs.Participants[pid]; ok {
		t.Errorf("participant should be deleted")
	}
	for _, tid := range tids {
		if _, ok := rs.Tracks[tid]; ok {
			t.Errorf("track %s should be deleted", tid)
		}
	}

	if len(deletedTIDs) != len(tids) {
		t.Errorf("expected %d deleted TIDs, got %d", len(tids), len(deletedTIDs))
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
	if track.TrackSID != tid || track.Kind != kind {
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

func TestRoomSnapshot_UpsertSubCache(t *testing.T) {
	rs := NewRoomSnapshot()
	rs.UpsertSubCache("t1", Paused)
	if rs.SubCache["t1"] != Paused {
		t.Errorf("expected true, got %v", rs.SubCache["t1"])
	}
}

func TestRoomSnapshot_RemoveSubCache(t *testing.T) {
	rs := NewRoomSnapshot()
	rs.UpsertSubCache("t1", Audio)

	rs.RemoveSubCache("t1")

	if _, ok := rs.SubCache["t1"]; ok {
		t.Fatal("expected subscription cache entry to be removed")
	}
}

func TestRoomSnapshot_ParticipantTracks(t *testing.T) {
	rs := NewRoomSnapshot()
	rs.UpsertParticipant("p1", "alice")
	_, err := rs.UpsertTrack("p1", "t1", TrackAudio)
	if err != nil {
		t.Fatalf("UpsertTrack() error = %v", err)
	}
	_, err = rs.UpsertTrack("p1", "t2", TrackVideo)
	if err != nil {
		t.Fatalf("UpsertTrack() error = %v", err)
	}
	rs.UpsertSubCache("t1", Paused)

	got, err := rs.ParticipantTracks("p1")
	if err != nil {
		t.Fatalf("ParticipantTracks() error = %v", err)
	}

	slices.Sort(got)
	want := []string{"t1", "t2"}
	if !slices.Equal(got, want) {
		t.Fatalf("ParticipantTracks() = %v, want %v", got, want)
	}
}

func TestRoomSnapshot_ParticipantTracks_MissingParticipant(t *testing.T) {
	rs := NewRoomSnapshot()

	got, err := rs.ParticipantTracks("missing")
	if err == nil {
		t.Fatal("expected error for missing participant")
	}
	if got != nil {
		t.Fatalf("expected nil tracks for missing participant, got %v", got)
	}
}

func TestRoomSnapshot_MultipleParticipantsAndTracks(t *testing.T) {
	rs := NewRoomSnapshot()
	// Add multiple participants and tracks
	for i := range 3 {
		pid := fmt.Sprintf("p%d", i)
		rs.UpsertParticipant(pid, fmt.Sprintf("user%d", i))

		kind := TrackAudio
		_, err := rs.UpsertTrack(pid, "t1", kind)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		kind = TrackVideo
		_, err = rs.UpsertTrack(pid, "t2", kind)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
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
