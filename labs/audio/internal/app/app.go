// Package app provides the CLI. Its only execution dependency is native Go.
package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/ssemakov/lkmcu/labs/audio/internal/course"
	"github.com/ssemakov/lkmcu/labs/audio/internal/lab"
)

const help = `Audio lab · Phase 1: the PCM model

Use ./audiolab from labs/audio; the launcher rebuilds through Go's cache.
Or replace audiolab below with: go run ./cmd/audiolab
Lesson commands use audiolab as shorthand for ./audiolab.

  lessons                       List the six lessons
  lesson N                      Explanation, predictions, task, and references
  hint N [1|2|3]                Reveal one hint (default: 1)
  check N|all                   Run native Go exercise tests; save check logs
  experiment SIGNAL [flags]     Generate PCM using your functions
  report [--path] [PATH]        Open latest/selected HTML report, or print its path
  chat N                        Print a context packet to paste into a chat
  help                          Show this help

Signals: silence, impulse, constant, sine, sandbox
Experiment flags:
  --duration 1s       Multiple of 20ms, from 20ms to 10s
  --frequency 1000    Sine frequency in Hz, below 24000
  --amplitude 0.5     Nonnegative linear amplitude (up to 16)
  --at 0              Impulse sample index
  --gain VALUE        Explicitly call your ApplyGain function
  --clip              Explicitly call your HardClip function
  --prediction TEXT   Save a prediction in the report
  --note TEXT         Save an observation in the report

Examples:
  audiolab lesson 1
  audiolab experiment impulse --duration 20ms --at 480
  audiolab experiment sine --amplitude 0.5 --gain 3 --clip
  audiolab report

Exercise functions start unfinished. Silence, impulse, and constant can be
plotted immediately; measurements stay Pending until you implement them.
Sine, gain, clipping, and sandbox require their respective implementations.
All runs are preserved under work/runs; work/ is ignored by Git.
`

func Run(args []string, out, errOut io.Writer) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		_, err := io.WriteString(out, help)
		return err
	}
	root, err := findRoot()
	if err != nil {
		return err
	}
	return run(root, args, out, errOut)
}

func run(root string, args []string, out, errOut io.Writer) error {
	switch args[0] {
	case "lessons":
		if len(args) != 1 {
			return fmt.Errorf("usage: audiolab lessons")
		}
		for _, l := range course.Lessons {
			fmt.Fprintf(out, "%d  %-28s exercises/%s\n", l.ID, l.Title, l.File)
		}
		fmt.Fprintln(out, "\nStart: audiolab lesson 1\nChecks are real tests, not completion markers. Unfinished checks fail.")
		return nil
	case "lesson", "hint", "chat":
		if len(args) < 2 || len(args) > 3 || (args[0] != "hint" && len(args) != 2) {
			suffix := ""
			if args[0] == "hint" {
				suffix = " [1|2|3]"
			}
			return fmt.Errorf("usage: audiolab %s N%s", args[0], suffix)
		}
		id, err := strconv.Atoi(args[1])
		if err != nil {
			return fmt.Errorf("lesson must be a number from 1 to 6")
		}
		l, err := course.Get(id)
		if err != nil {
			return err
		}
		if args[0] == "chat" {
			return chat(root, l, out)
		}
		if args[0] == "hint" {
			level := 1
			if len(args) == 3 {
				level, err = strconv.Atoi(args[2])
				if err != nil || level < 1 || level > len(l.Hints) {
					return fmt.Errorf("hint level must be 1, 2, or 3")
				}
			}
			fmt.Fprintf(out, "Lesson %d · %s · Hint %d\n\n%s\n", l.ID, l.Title, level, l.Hints[level-1])
			return nil
		}
		content, err := l.Text()
		if err != nil {
			return err
		}
		fmt.Fprintln(out, content)
		fmt.Fprintf(out, "\nEdit: %s\nHints: audiolab hint %d [1|2|3]\n", filepath.Join(root, "exercises", l.File), l.ID)
		return nil
	case "check":
		if len(args) != 2 {
			return fmt.Errorf("usage: audiolab check N|all")
		}
		selected := course.Lessons
		if args[1] != "all" {
			id, err := strconv.Atoi(args[1])
			if err != nil {
				return fmt.Errorf("choose a lesson number or all")
			}
			l, err := course.Get(id)
			if err != nil {
				return err
			}
			selected = []course.Lesson{l}
		}
		return check(root, selected, out, errOut)
	case "experiment":
		return experiment(root, args[1:], out, errOut)
	case "report":
		return report(root, args[1:], out, errOut)
	default:
		return fmt.Errorf("unknown command %q; run audiolab help", args[0])
	}
}

func findRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		data, readErr := os.ReadFile(filepath.Join(dir, "go.mod"))
		if readErr == nil && strings.TrimSpace(strings.SplitN(string(data), "\n", 2)[0]) == "module github.com/ssemakov/lkmcu/labs/audio" {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("run from the labs/audio directory (or one of its subdirectories)")
		}
		dir = parent
	}
}

func check(root string, lessons []course.Lesson, out, errOut io.Writer) error {
	logDir := filepath.Join(root, "work", "checks")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return err
	}
	failures := 0
	for _, l := range lessons {
		fmt.Fprintf(out, "\n── Lesson %d · %s ──\n", l.ID, l.Title)
		var log bytes.Buffer
		fmt.Fprintf(&log, "Checked %s\n", time.Now().Format(time.RFC3339))
		cmd := exec.Command("go", "test", "./exercises", "-run", l.TestPattern(), "-count=1", "-v")
		cmd.Dir = root
		cmd.Stdout = io.MultiWriter(out, &log)
		cmd.Stderr = cmd.Stdout
		runErr := cmd.Run()
		if err := os.WriteFile(filepath.Join(logDir, fmt.Sprintf("%02d.txt", l.ID)), log.Bytes(), 0o644); err != nil {
			return err
		}
		if runErr != nil {
			var exit *exec.ExitError
			if !errors.As(runErr, &exit) {
				return fmt.Errorf("could not run native Go: %w", runErr)
			}
			failures++
		}
	}
	fmt.Fprintf(out, "\n%d/%d lesson checks passed. Latest logs: %s\n", len(lessons)-failures, len(lessons), logDir)
	if failures > 0 {
		return fmt.Errorf("%d lesson check(s) failed; unfinished exercise functions are expected to fail", failures)
	}
	return nil
}

func experiment(root string, args []string, out, errOut io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: audiolab experiment SIGNAL [flags]; run audiolab help")
	}
	if args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(out, help)
		return nil
	}
	c := lab.Config{Signal: args[0]}
	fs := flag.NewFlagSet("experiment "+c.Signal, flag.ContinueOnError)
	fs.SetOutput(errOut)
	fs.DurationVar(&c.Duration, "duration", time.Second, "multiple of 20ms, up to 10s")
	fs.Float64Var(&c.Frequency, "frequency", 1000, "sine frequency in Hz")
	fs.Float64Var(&c.Amplitude, "amplitude", 0.5, "linear amplitude")
	fs.IntVar(&c.ImpulseAt, "at", 0, "impulse sample index")
	gain := fs.Float64("gain", 1, "explicit gain stage")
	fs.BoolVar(&c.Clip, "clip", false, "explicit hard-clip stage")
	fs.StringVar(&c.Prediction, "prediction", "", "prediction to save")
	fs.StringVar(&c.Note, "note", "", "observation to save")
	if err := fs.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %s (put SIGNAL before flags)", strings.Join(fs.Args(), " "))
	}
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "gain" {
			c.Gain = gain
		}
	})
	e, err := lab.Build(c, lab.LearnerFunctions())
	if err != nil {
		return err
	}
	path, err := lab.Save(root, e)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "%s · %s · %d samples · %d blocks\n", c.Signal, c.Duration, len(e.Stages[0].Samples), len(e.Stages[0].Samples)/lab.BlockSamples)
	for _, s := range e.Stages {
		fmt.Fprintf(out, "  %-14s peak %-9s RMS %-9s peak dBFS %-9s outside ±1: %d\n", s.Name, s.Peak.Display, s.RMS.Display, s.PeakDBFS.Display, s.OverRange)
	}
	fmt.Fprintf(out, "\nShared listening gain: %.6g (original PCM is in data.json)\nReport: %s\nOpen: audiolab report\n", e.PlaybackGain, path)
	return nil
}

func report(root string, args []string, out, errOut io.Writer) error {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	fs.SetOutput(errOut)
	pathOnly := fs.Bool("path", false, "print the path without opening a browser")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() > 1 {
		return fmt.Errorf("usage: audiolab report [--path] [PATH]")
	}
	path := ""
	if fs.NArg() == 1 {
		path = fs.Arg(0)
	} else {
		var err error
		path, err = latest(root)
		if err != nil {
			return err
		}
	}
	path, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	stat, err := os.Stat(path)
	if err != nil {
		return err
	}
	if stat.IsDir() || filepath.Ext(path) != ".html" {
		return fmt.Errorf("choose a report.html file")
	}
	fmt.Fprintln(out, path)
	if *pathOnly {
		return nil
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", path)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("open this local HTML file in a browser: %s (%w)", path, err)
	}
	return nil
}

func latest(root string) (string, error) {
	data, err := os.ReadFile(filepath.Join(root, "work", "latest"))
	if errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("no report yet; try audiolab experiment impulse --duration 20ms")
	}
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func chat(root string, l course.Lesson, out io.Writer) error {
	code, err := os.ReadFile(filepath.Join(root, "exercises", l.File))
	if err != nil {
		return err
	}
	text, err := l.Text()
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "I'm studying Phase 1 PCM in Go, lesson %d: %s.\nHelp me reason about the code. Start with questions or a conceptual hint; show a full implementation only if I ask.\n\nLesson:\n%s\nMy current code:\n\n%s\n", l.ID, l.Title, text, code)
	if data, err := os.ReadFile(filepath.Join(root, "work", "checks", fmt.Sprintf("%02d.txt", l.ID))); err == nil {
		fmt.Fprintf(out, "\nMost recent check (may predate current edits):\n%s\n", data)
	} else {
		fmt.Fprintln(out, "\nNo saved check for this lesson. Run audiolab check first.")
	}
	if path, err := latest(root); err == nil {
		if raw, err := os.ReadFile(filepath.Join(filepath.Dir(path), "data.json")); err == nil {
			var e lab.Experiment
			if json.Unmarshal(raw, &e) == nil {
				fmt.Fprintf(out, "\nLatest experiment snapshot (may be from a different lesson):\nSignal: %s, duration: %s, frequency: %g Hz, amplitude: %g, clip: %t\n", e.Config.Signal, e.Config.Duration, e.Config.Frequency, e.Config.Amplitude, e.Config.Clip)
				if e.Config.Gain != nil {
					fmt.Fprintf(out, "Gain: %g\n", *e.Config.Gain)
				}
				fmt.Fprintf(out, "Prediction: %s\nObservation: %s\n", e.Config.Prediction, e.Config.Note)
				for _, s := range e.Stages {
					fmt.Fprintf(out, "%s: peak=%s RMS=%s sample-peak dBFS=%s over-range=%d\n", s.Name, s.Peak.Display, s.RMS.Display, s.PeakDBFS.Display, s.OverRange)
				}
			}
		}
	}
	notePath := "notes/observations.md"
	if l.ID == 6 {
		notePath = "notes/06-pcm-contract.md"
	}
	if data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(notePath))); err == nil {
		fmt.Fprintf(out, "\nMy notes (%s):\n%s\n", notePath, data)
	}
	fmt.Fprintln(out, "\nMy question:\n[Write the specific question here before pasting.]")
	return nil
}
