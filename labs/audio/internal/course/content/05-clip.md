# 5 · Beyond full scale

Multiply an amplitude-0.5 sine by 3: its new amplitude is 1.5. Float32 stores
those values without clipping. An explicit hard clip replaces excursions
outside [-1,+1] with the nearest boundary. That changes waveform shape and
introduces distortion. It is a useful baseline, not our eventual limiter.

Predict before coding:
- What will peak and RMS do under gain 3 before clipping?
- Which samples will change when hard clipping is applied?
- Will lowering playback volume after clipping restore the original shape?

Implement HardClip in exercises/05_clip.go. Keep the input untouched.
Check: audiolab check 5

Explore:
  audiolab experiment sine --frequency 1000 --amplitude 0.5 --gain 3 --clip --prediction "The gained waveform reaches 1.5; clipping flattens its peaks"
  audiolab report

The report retains input, gained, and explicitly clipped stages. Listen to
the gained and clipped versions at the SAME shared attenuation. The gained
version is brought into playback range linearly; the clipped version keeps
its distorted shape. WAV files are listening copies, not the raw PCM data.
data.json preserves every original sample and each measurement status.

Repeat with an impulse and a constant. With your Sandbox function, try a
signal that overloads only briefly. Clipping after gain is intentionally
explicit: without --clip the runner does not call HardClip.

Checkpoint: explain why values over ±1 do not prove that clipping has already
happened, and why summing first and managing output range later is useful.

Reference: https://wiki.xiph.org/Videos/Digital_Show_and_Tell
