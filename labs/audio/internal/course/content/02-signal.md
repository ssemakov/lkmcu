# 2 · What samples represent

Zero is the waveform's zero reference; all-zero PCM is digital silence.
Negative samples are the opposite polarity, not "negative loudness". A
constant nonzero value is a DC offset, not a sustained tone. You may hear
its start/stop transients, and playback hardware may remove the DC component.

Our normalized scale uses ±1 as nominal full scale. Float32 itself can store
much larger magnitudes; crossing ±1 does not automatically change a number.
Frequency controls cycles per second; amplitude controls the excursions.

Predict before coding:
- At 1 kHz and 48 kHz sampling, how many samples form one cycle?
- Where are the first positive peak, zero crossing, and negative peak?
- Will resetting phase every 960 samples make a continuous 440 Hz signal?

Implement Sine in exercises/02_signal.go. Generate one continuous sequence
from sample indices, including when the requested length spans many blocks.
Check: audiolab check 2

Explore all four shapes:
  audiolab experiment silence --duration 20ms
  audiolab experiment impulse --duration 20ms --at 12 --amplitude 0.5
  audiolab experiment constant --amplitude 0.5
  audiolab experiment sine --frequency 1000 --amplitude 0.5
  audiolab report

Each experiment saves a separate report. Use audiolab report PATH to reopen
an older one. The default duration is one second for listening; the waveform
starts by showing one 960-sample block. Try 1 ms zoom and inspect sample 12.
Plot lines only connect stored samples; they are not analog reconstruction.

Checkpoint: recognize silence, an impulse, DC, and a sine from their samples.
Change the sine to 440 Hz and inspect the boundary around sample 960.

Reference: https://digital-media-study.vercel.app/waves
