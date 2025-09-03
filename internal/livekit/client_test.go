package livekit

import (
	"testing"

	lksdk "github.com/livekit/server-sdk-go/v2"
	"github.com/pion/webrtc/v4"
)

func TestNewClient(t *testing.T) {
	url := "ws://localhost:7880"
	apiKey := "test-key"
	apiSecret := "test-secret"

	client := NewClient(url, apiKey, apiSecret)

	if client == nil {
		t.Fatal("NewClient returned nil")
	}
	if client.url != url {
		t.Errorf("want url=%s, got %s", url, client.url)
	}
	if client.apiKey != apiKey {
		t.Errorf("want apiKey=%s, got %s", apiKey, client.apiKey)
	}
	if client.apiSecret != apiSecret {
		t.Errorf("want apiSecret=%s, got %s", apiSecret, client.apiSecret)
	}
	if client.room != nil {
		t.Error("want room=nil, got non-nil room")
	}
}

func TestNewClient_EmptyValues(t *testing.T) {
	client := NewClient("", "", "")

	if client == nil {
		t.Fatal("NewClient returned nil")
	}
	if client.url != "" {
		t.Errorf("want empty url, got %s", client.url)
	}
	if client.apiKey != "" {
		t.Errorf("want empty apiKey, got %s", client.apiKey)
	}
	if client.apiSecret != "" {
		t.Errorf("want empty apiSecret, got %s", client.apiSecret)
	}
	if client.room != nil {
		t.Error("want room=nil, got non-nil room")
	}
}

func TestClient_Close_WithoutRoom(t *testing.T) {
	client := NewClient("ws://localhost:7880", "key", "secret")

	err := client.Close()
	if err != nil {
		t.Errorf("Close() returned error: %v", err)
	}
}

func TestClient_Close_WithRoom(t *testing.T) {
	client := NewClient("ws://localhost:7880", "key", "secret")

	// Create a mock room - in a real scenario this would be set by JoinRoom
	// For testing purposes, we'll simulate having a room
	mockRoom := &mockRoom{}
	client.room = mockRoom

	err := client.Close()
	if err != nil {
		t.Errorf("Close() returned error: %v", err)
	}

	if !mockRoom.disconnectCalled {
		t.Error("expected Disconnect() to be called on room")
	}
}

func TestTrackSubscribed(t *testing.T) {
	// Test that the callback doesn't panic with nil inputs
	trackSubscribed(nil, nil, nil)

	// Test with valid but empty track/publication/participant
	// Since the function is currently empty (TODO), we just ensure it doesn't panic
	track := &webrtc.TrackRemote{}
	pub := &lksdk.RemoteTrackPublication{}
	participant := &lksdk.RemoteParticipant{}

	trackSubscribed(track, pub, participant)
}

// mockRoom implements the minimal interface needed for testing
type mockRoom struct {
	disconnectCalled bool
}

func (m *mockRoom) Disconnect() {
	m.disconnectCalled = true
}
