package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ssemakov/lkmcu/labs/audio/internal/course"
)

func TestAllLessonsAndHintLevelsAreAvailable(t *testing.T) {
	for _, lesson := range course.Lessons {
		var out bytes.Buffer
		if err := run(t.TempDir(), []string{"lesson", string(rune('0' + lesson.ID))}, &out, &out); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), lesson.File) || !strings.Contains(out.String(), "Checkpoint") && lesson.ID != 6 {
			t.Errorf("lesson %d missing instructions", lesson.ID)
		}
		if len(lesson.Hints) != 3 {
			t.Errorf("lesson %d missing hint levels", lesson.ID)
		}
	}
}

func TestCLIRejectsInvalidArgumentsBeforeCreatingWork(t *testing.T) {
	for _, args := range [][]string{
		{"lesson", "0"}, {"lesson", "7"}, {"hint", "3", "4"}, {"check", "unknown"},
		{"experiment"}, {"experiment", "sine", "--frequency", "NaN"},
		{"experiment", "sine", "--duration", "25ms"},
		{"experiment", "silence", "extra"}, {"experiment", "silence", "--unknown"},
		{"report", "--path"}, {"not-a-command"},
	} {
		root := t.TempDir()
		var out bytes.Buffer
		if err := run(root, args, &out, &out); err == nil {
			t.Errorf("accepted invalid arguments %v", args)
		}
		if _, err := os.Stat(filepath.Join(root, "work")); !os.IsNotExist(err) {
			t.Errorf("%v created work before validation", args)
		}
	}
}

func TestExperimentReportAndChatWorkTogether(t *testing.T) {
	root := t.TempDir()
	var out bytes.Buffer
	args := []string{"experiment", "impulse", "--duration", "20ms", "--at", "480", "--prediction", "At 10 ms", "--note", "Inspect the impulse"}
	if err := run(root, args, &out, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "960 samples") {
		t.Fatal("experiment summary missing sample count")
	}
	out.Reset()
	if err := run(root, []string{"report", "--path"}, &out, &out); err != nil {
		t.Fatal(err)
	}
	path := strings.TrimSpace(out.String())
	if _, err := os.Stat(path); err != nil {
		t.Fatal("report command did not locate generated HTML")
	}
	if err := os.MkdirAll(filepath.Join(root, "exercises"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "exercises", "03_levels.go"), []byte("// current learner code"), 0o644); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := chat(root, course.Lessons[2], &out); err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"current learner code", "At 10 ms", "Inspect the impulse", "No saved check", "full implementation only if I ask"} {
		if !strings.Contains(out.String(), text) {
			t.Errorf("chat packet missing %q", text)
		}
	}
}
