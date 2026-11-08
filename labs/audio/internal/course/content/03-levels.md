# 3 · Measuring a block

Sample peak is max(abs(sample)). RMS is sqrt(mean(sample²)). Peak tells you
the largest stored excursion. RMS describes average squared amplitude over
the measurement window; it is not a perceptual loudness measurement.

For a sine of amplitude A over complete cycles, RMS is A/sqrt(2). An impulse
and a sustained signal can have the same peak with very different RMS.
Averaging signed samples can give zero for a loud symmetric waveform.

Predict before coding:
- What are peak and RMS for [0, 0, 0, 0]?
- For [0.5, 0.5, 0.5, 0.5]? For [1, 0, 0, 0]?
- For [-0.5, 0.5, -0.5, 0.5]? Why is its mean misleading?

Implement Peak and RMS in exercises/03_levels.go. Empty input returns zero.
Use float64 intermediate arithmetic; squaring a float32 first can overflow.
Check: audiolab check 3

Explore and compare reports:
  audiolab experiment impulse --duration 20ms
  audiolab experiment constant --duration 20ms --amplitude 1
  audiolab experiment sine --frequency 1000 --amplitude 0.5

The report metrics cover the ENTIRE generated signal, independent of chart
zoom. A 1 kHz, amplitude-0.5 sine should have peak 0.5 and RMS about 0.353553.
The report calls your functions; pending or invalid measurements are labeled.
Changing your code requires rerunning the experiment to make a new report.

Checkpoint: describe a case where peak alone does not describe the signal's
sustained level. Predict what making the impulse's measurement window longer
does to its peak and RMS, then test it.

Reference: https://digital-media-study.vercel.app/primer
