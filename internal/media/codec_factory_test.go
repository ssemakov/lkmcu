package media

import (
	"testing"
)

func TestDefaultCodecFactory_NewDepaketizer(t *testing.T) {
	factory := &DefaultCodecFactory{}

	tests := []struct {
		name     string
		mimeType string
		wantErr  bool
	}{
		{"Opus Depaketizer", "audio/opus", false},
		{"Unsupported Depaketizer", "video/vp8", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			depak, err := factory.NewDepaketizer(tt.mimeType)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewDepaketizer() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && depak == nil {
				t.Errorf("Expected depaketizer, got nil")
			}
		})
	}
}

func TestDefaultCodecFactory_NewDecoder(t *testing.T) {
	factory := &DefaultCodecFactory{}

	tests := []struct {
		name     string
		mimeType string
		wantErr  bool
	}{
		{"Opus Decoder", "audio/opus", false},
		{"Unsupported Decoder", "video/vp8", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dec, err := factory.NewDecoder(tt.mimeType)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewDecoder() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && dec == nil {
				t.Errorf("Expected decoder, got nil")
			}
		})
	}
}
