# 1 · Samples and time

A PCM stream is a sequence of signed numbers at regularly spaced instants.
48,000 Hz means 48,000 samples per second per channel. The sample period is
1/48,000 second, about 20.833 microseconds. Sample i is at time i/48,000;
the duration covered by N sample slots is N/48,000 seconds.

A frame contains one sample for every channel at the same instant. Mono has
one value per frame. Interleaved stereo is [L0, R0, L1, R1, ...], with two
values per frame. Do not multiply samples PER CHANNEL by the channel count.

Predict before coding:
- How many samples per channel occupy 20 ms at 48 kHz? How many at 10 ms?
- How many scalar float32 values would 20 ms of interleaved stereo contain?
- Why would converting 20 ms to integer seconds first give the wrong answer?

Implement SamplesForDuration in exercises/01_time.go.
Check: audiolab check 1
Explore: audiolab experiment impulse --duration 20ms --at 480 --prediction "The impulse should appear at 10 ms"
Open: audiolab report

The runner supplies the fixed 48 kHz / 960-sample target preset. It does not
quietly implement or call your time exercise. Measurements remain pending
until you implement them in lesson 3. Inspect sample 480 in the report.

Checkpoint: explain the difference between a frame, a sample per channel, and
a scalar value in an interleaved buffer. Write one sentence in your notebook.

Reference: https://digital-media-study.vercel.app/sampling
Further background: https://wiki.xiph.org/Videos/Digital_Show_and_Tell
