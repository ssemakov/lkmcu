# Audio lab · Phase 1

A native Go lab for the PCM model behind the N-source audio mixer in
[issue #9](https://github.com/ssemakov/lkmcu/issues/9).
This is a separate Go module with no third-party dependencies. Unfinished
exercises do not participate in the parent module's test run.

## Start

Install Go 1.25 or newer. From this directory:

```sh
./audiolab lessons
./audiolab lesson 1
# Edit exercises/01_time.go in your editor.
./audiolab check 1
```

The launcher rebuilds the binary through Go's build cache every time, so
experiments use your current code. You can also run
`go run ./cmd/audiolab lesson 1`. On Windows, use that form instead of the shell
launcher. Commands inside lessons use `audiolab` as shorthand for `./audiolab`.

The exercise bodies intentionally panic with `Unimplemented`. Replace those
panics with your work. A failed exercise check is expected at the start;
tests are never skipped or marked passed just because an exercise is pending.
There are no reference solutions in this module.

## The learning loop

1. Read a lesson and make its predictions before coding.
2. Implement its functions in `exercises/`.
3. Run `./audiolab check N`, or invoke ordinary Go tests directly:
   `go test ./exercises -run '^TestLesson01_' -count=1 -v`.
4. Run an experiment, inspect samples, and listen.
5. Record an observation and its implication for the mixer in
   `notes/observations.md` or with experiment flags.
6. Request a hint or paste a context packet into your existing chat when useful.

Aim for two sessions of 45–60 minutes. If stuck, use hint 1, then 2, then 3.
Hints reveal reasoning and pseudocode; they do not edit your code.

| Lesson | File | Work |
| --- | --- | --- |
| 1 · Samples and time | `exercises/01_time.go` | Duration to samples per channel; mono vs interleaved stereo |
| 2 · What samples represent | `exercises/02_signal.go` | Continuous sine generation from sample indices |
| 3 · Measuring a block | `exercises/03_levels.go` | Peak and RMS, including silence and impulses |
| 4 · Gain and dBFS | `exercises/04_gain.go` | Non-mutating gain and amplitude-to-dB conversion |
| 5 · Beyond full scale | `exercises/05_clip.go` | Non-mutating hard clipping as a distortion baseline |
| 6 · PCM contract | `exercises/06_contract.go` | Format values plus your paragraph in `notes/06-pcm-contract.md` |

`exercises/sandbox.go` is an optional open-ended signal generator. Try a pulse
train, DC plus a sine, or a signal that overloads briefly.

## Experiments and reports

You can inspect the supplied silence, impulse, and constant signals immediately:

```sh
./audiolab experiment impulse --duration 20ms --at 480 \
  --prediction "The impulse should appear at 10 ms"
./audiolab report
```

Peak, RMS, and dBFS show **Pending** until you implement those functions.
They have no reference fallbacks. Sine generation, an explicit gain stage,
hard clipping, and sandbox require their respective learner implementations.
Other panics or non-finite measurement results are labeled errors. Invalid
sample counts or NaN/Inf samples stop an experiment with an actionable error.

After the relevant lessons:

```sh
./audiolab experiment sine --frequency 1000 --amplitude 0.5
./audiolab experiment sine --frequency 440 --amplitude 0.5 --gain -1
./audiolab experiment sine --amplitude 0.5 --gain 3 --clip \
  --prediction "The gained samples exceed full scale" \
  --note "Compare the gained and clipped shapes at the same playback gain"
./audiolab report
```

The default duration is **one second** for listening, with an initial chart
window of **20 ms / 960 samples**. Duration must be a multiple of 20 ms, up to
10 seconds. Waveform controls select a block, sample offset, and 20/5/1 ms
window. Hover or use the keyboard-accessible sample cursor to inspect values.
Stage checkboxes let you compare overlays. Lines connect stored samples;
the chart does not perform analog reconstruction or measure true peaks.

Metrics cover the **whole generated signal**, independent of chart zoom.
Peak, RMS, and dBFS come from your functions. The runner separately counts
over-range samples and scans the data to choose safe listening attenuation;
those operations do not substitute answers for your metrics. The runner's
fixed 48 kHz / 960-sample preset is supplied scaffolding, independent of your
lesson 1 calculation and lesson 6 format.

Every run is a snapshot saved under ignored `work/runs/`:

- `report.html`: local viewer with embedded original samples; no server,
  internet connection, npm install, or external assets are needed.
- `data.json`: original float32 sample values, settings, predictions,
  observations, and measurement states. Negative-infinite dBFS is represented
  by a ready metric with a display value and no finite numeric value.
- `input.wav`, and optionally `gained.wav` and `clipped.wav`: mono PCM16
  **listening copies**. All stages in one report share the same labeled gain
  (at most 0.1, reduced further when needed). They are not independently
  normalized or silently clipped during export. The explicit `--clip` stage
  calls your HardClip function before listening attenuation.

Keep the report folder together, because audio and the JSON download are
neighboring files. Plots and metrics retain original amplitudes. Quantization
and attenuation in a listening WAV mean it is not a raw PCM export. Playback
of a nonzero constant can produce boundary clicks; it is not a steady tone.

```sh
./audiolab report --path                    # Print the latest report path
./audiolab report work/runs/OLDER/report.html
./audiolab hint 3                           # First hint only
./audiolab hint 3 2                         # Stronger hint
./audiolab check all                        # Six independent lesson checks
./audiolab chat 3                           # Print context; nothing is sent
./audiolab chat 3 | pbcopy                  # Optional macOS clipboard handoff
```

Check logs are saved under `work/checks/`. Chat context includes the lesson,
current exercise code, latest check, latest experiment summary, and your
notes. Logs and reports may predate edits; they are labeled as snapshots.
The tool has no embedded LLM or API keys and does not contact a chat service.

## Completion and maintenance

Finish when you can explain 960 samples per block, predict simple gain and
level behavior, distinguish floating-point headroom from clipping, and defend
your PCM contract. Lesson 6's test checks fields; reviewing your paragraph
and reasoning is a human checkpoint.

For lab infrastructure checks, independent of unfinished exercises:

```sh
go test ./internal/... ./cmd/...
go vet ./...
```

`go test ./...` inside **this** module also runs the learner tests and is
expected to fail until you finish. The parent MCU module's `go test ./...`
does not descend into this module. The next learning increment is aligned
summation, preservation, doubling, cancellation, and silence for Phase 2.
