package layout

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLayoutView_UpdateFromStage(t *testing.T) {
	lv := DefaultLayoutView()

	stage := &Stage{Slots: []Slot{{Identity: "alice", TrackKind: "video"}}}
	lv.UpdateFromStage(stage)

	if lv.CurrentStage != stage {
		t.Errorf("want CurrentStage pointer updated")
	}

	kinds, ok := lv.StageParticipants["alice"]

	if !ok {
		t.Fatalf("want alice present in StageParticipants")
	}
	if len(kinds) != 1 || kinds[0] != "video" {
		t.Fatalf("unexpected kinds for alice: %#v", kinds)
	}
}

func TestLayoutView_Snapshot(t *testing.T) {
	lv := DefaultLayoutView()
	lv.StageTracks["track1"] = "alice"
	stage := &Stage{Slots: []Slot{{Identity: "alice", TrackKind: "video"}}}
	lv.UpdateFromStage(stage)

	snap := lv.Snapshot()

	if snap.CurrentStage != lv.CurrentStage {
		t.Errorf("want snapshot to keep stage pointer")
	}

	if len(snap.StageParticipants) != 1 {
		t.Fatalf("unexpected snapshot StageParticipants: %#v", snap.StageParticipants)
	}

	snap.StageParticipants["alice"][0] = "audio"
	if lv.StageParticipants["alice"][0] != "video" {
		t.Fatalf("expected original StageParticipants to remain unchanged")
	}

	snap.StageTracks["track1"] = "bob"
	if lv.StageTracks["track1"] != "alice" {
		t.Fatalf("expected original StageTracks to remain unchanged")
	}
}

func TestHTTPHandlerPost_InvokesCallback(t *testing.T) {
	stageForGet := &Stage{}

	reqBody := map[string]any{
		"slots": []map[string]any{{
			"identity":  "bob",
			"trackKind": "video",
		}},
	}
	b, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/layout", bytes.NewReader(b))
	resp := httptest.NewRecorder()

	var received *Stage
	callback := func(s *Stage) {
		received = s
	}

	if err := HTTPHandler(stageForGet, callback, resp, req); err != nil {
		t.Fatalf("HTTPHandler returned error: %v", err)
	}

	if resp.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", resp.Code)
	}

	if received == nil {
		t.Fatalf("expected callback to receive stage")
	}

	if len(received.Slots) != 1 || received.Slots[0].Identity != "bob" {
		t.Fatalf("unexpected stage passed to callback: %#v", received)
	}

	if len(stageForGet.Slots) != 0 {
		t.Fatalf("expected original stage pointer to remain unchanged")
	}
}

func TestHTTPHandler_InvalidLayout(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/layout", bytes.NewBufferString(`{"slots":[]}`))
	resp := httptest.NewRecorder()

	called := false
	callback := func(_ *Stage) { called = true }

	_ = HTTPHandler(DefaultStage(), callback, resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", resp.Code)
	}

	if called {
		t.Fatalf("callback should not be invoked for invalid layout")
	}
}
