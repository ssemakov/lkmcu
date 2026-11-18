Mixer PCM contract:

Sample rate: 44800 kHz
Channels: 1
Layout: mono, we are not going to convert stereo tracks into mono at this version instead we'll just reject them. The LK default for microphone is 'mono'
Sample Type: float32
Block Duration: 20 ms
Samples Per Channel: 960
Allow Over Range: yes, the mixer should internally allow over range values. A later gain or limiter stage should bring output into range.
Allow Non Finite: no, every sample must be a finite number

- Where output range management belongs.
- What a sample-peak measurement does not guarantee about reconstructed peaks.
