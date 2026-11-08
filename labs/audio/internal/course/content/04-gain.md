# 4 · Gain and dBFS

Linear gain multiplies every sample. Gain 0 makes silence; 0.5 halves the
amplitude; -1 reverses polarity without changing peak or RMS. Preserve the
original buffer so experiments can compare the exact before/after samples.

For amplitude ratios, dB = 20*log10(abs(amplitude)/reference). We use reference
1.0. Applied to sample peak, this is sample-peak dBFS: 1 → 0 dBFS,
0.5 → approximately -6.02 dBFS, 2 → approximately +6.02 dBFS.
Zero is -Inf dBFS, not 0 dBFS. Positive dBFS is possible in internal float PCM.
For a below-full-scale peak, headroom to ±1 is the negative of its dBFS value.

The report shows RMS in LINEAR units to avoid RMS dBFS reference conventions.
Equal RMS also does not imply equal perceived loudness for arbitrary sounds.

Predict before coding:
- What happens to the peak and RMS of amplitude-0.5 sine under gain 2?
- What changes under gain -1? Which measurements should stay the same?
- How many dB of sample-peak headroom does amplitude 0.25 have?

Implement ApplyGain and AmplitudeDB in exercises/04_gain.go.
Check: audiolab check 4

Explore:
  audiolab experiment sine --amplitude 0.5 --gain 0.5
  audiolab experiment sine --amplitude 0.5 --gain 2
  audiolab experiment sine --amplitude 0.5 --gain -1
  audiolab experiment silence --gain 3

Use the report's overlaid curves and inspect a sample. All listening copies
in ONE report share the same labeled playback attenuation. Plots and metrics
use original amplitudes. Different reports may use different shared gains;
use their labels when comparing listening levels.

Checkpoint: explain why polarity inversion and attenuation are different,
and why 0 dBFS does not mean silence.

Reference: https://digital-media-study.vercel.app/waves
