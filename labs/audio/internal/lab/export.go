package lab

import (
	"bytes"
	_ "embed"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"math"
	"os"
	"path/filepath"
	"time"
)

//go:embed report.html
var reportTemplate string

func RenderReport(w io.Writer, e *Experiment) error {
	data, err := json.Marshal(e) // JSON's HTML escaping protects the script boundary.
	if err != nil {
		return err
	}
	t, err := template.New("report").Parse(reportTemplate)
	if err != nil {
		return err
	}
	return t.Execute(w, struct {
		Experiment *Experiment
		Data       template.JS
		Duration   string
		PlaybackDB string
	}{e, template.JS(data), e.Config.Duration.String(), fmt.Sprintf("%.2f", 20*math.Log10(e.PlaybackGain))})
}

// WriteWAV exports mono PCM16 listening copies. Reject over-range data rather
// than silently clipping; the shared playback gain must already fit all stages.
func WriteWAV(w io.Writer, samples []float32, gain float64) error {
	if !finite(gain) || gain <= 0 || len(samples) > SampleRate*10 {
		return fmt.Errorf("invalid listening copy size or gain")
	}
	var pcm bytes.Buffer
	for i, v := range samples {
		scaled := float64(v) * gain
		if !finite(scaled) || math.Abs(scaled) > 1 {
			return fmt.Errorf("listening sample %d is out of range", i)
		}
		if err := binary.Write(&pcm, binary.LittleEndian, int16(math.Round(scaled*32767))); err != nil {
			return err
		}
	}
	size := uint32(pcm.Len())
	var header bytes.Buffer
	header.WriteString("RIFF")
	_ = binary.Write(&header, binary.LittleEndian, uint32(36)+size)
	header.WriteString("WAVEfmt ")
	for _, v := range []any{uint32(16), uint16(1), uint16(1), uint32(SampleRate), uint32(SampleRate * 2), uint16(2), uint16(16)} {
		_ = binary.Write(&header, binary.LittleEndian, v)
	}
	header.WriteString("data")
	_ = binary.Write(&header, binary.LittleEndian, size)
	if _, err := w.Write(header.Bytes()); err != nil {
		return err
	}
	_, err := w.Write(pcm.Bytes())
	return err
}

// Save preserves every run, then atomically updates the latest-report pointer.
func Save(root string, e *Experiment) (report string, err error) {
	runs := filepath.Join(root, "work", "runs")
	if err := os.MkdirAll(runs, 0o755); err != nil {
		return "", err
	}
	dir, err := os.MkdirTemp(runs, time.Now().Format("20060102-150405")+"-"+e.Config.Signal+"-")
	if err != nil {
		return "", err
	}
	defer func() {
		if err != nil {
			_ = os.RemoveAll(dir) // Only the new, incomplete run we just created.
		}
	}()
	data, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return "", err
	}
	if err = os.WriteFile(filepath.Join(dir, "data.json"), data, 0o644); err != nil {
		return "", err
	}
	for _, s := range e.Stages {
		var wav bytes.Buffer
		if err = WriteWAV(&wav, s.Samples, e.PlaybackGain); err != nil {
			return "", err
		}
		if err = os.WriteFile(filepath.Join(dir, s.WAV), wav.Bytes(), 0o644); err != nil {
			return "", err
		}
	}
	var html bytes.Buffer
	if err = RenderReport(&html, e); err != nil {
		return "", err
	}
	report = filepath.Join(dir, "report.html")
	if err = os.WriteFile(report, html.Bytes(), 0o644); err != nil {
		return "", err
	}
	pointer, err := os.CreateTemp(filepath.Join(root, "work"), "latest-*.tmp")
	if err != nil {
		return "", err
	}
	defer os.Remove(pointer.Name())
	_, writeErr := pointer.WriteString(report)
	closeErr := pointer.Close()
	if writeErr != nil {
		return "", writeErr
	}
	if closeErr != nil {
		return "", closeErr
	}
	err = os.Rename(pointer.Name(), filepath.Join(root, "work", "latest"))
	return report, err
}
