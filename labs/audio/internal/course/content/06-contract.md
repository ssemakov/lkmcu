# 6 · Your PCM contract

Make the Phase 1 boundary precise: 48,000 samples/second, one channel, a mono
sequence of float32 values, 20 ms per block, 960 samples per channel. Permit
finite internal excursions beyond ±1 so later summation can preserve them.
NaN and infinity are not meaningful audio samples. Output range management
belongs in an explicit downstream stage.

Sample peaks and true peaks answer different questions. Stored samples do
not capture every extremum of a reconstructed continuous waveform. For
example, a 12 kHz sine at 48 kHz with phase π/4 has repeating stored values
approximately [+0.707, +0.707, -0.707, -0.707], while the underlying unit sine
still reaches ±1 between some sample instants. Scaling those stored values
to a sample peak of 1 would make that sine's continuous peak about 1.414.
Sample-peak bounds alone therefore do not promise reconstruction-peak bounds.
Do not implement oversampling or true-peak limiting in Phase 1.

Predict and explain:
- Can a stream with all stored samples within ±1 still have intersample peaks
  above ±1 after reconstruction?
- Does joining two 20 ms blocks change the rate or double the block size?
- At what boundary should a mixer promise its final output ceiling?

Implement MixerFormat in exercises/06_contract.go.
Write one paragraph in notes/06-pcm-contract.md. Tests check the returned
fields; you review the paragraph's reasoning yourself or discuss it in chat.
Check: audiolab check 6
Review all checks: audiolab check all
Prepare chat context: audiolab chat 6

Optional sandbox experiment: synthesize the phase-shifted 12 kHz example
above and inspect its stored samples. The plot connects samples for
inspection; it does NOT draw band-limited reconstruction or measure true peak.

Phase 1 is finished when you can explain the format, predict basic gain and
level behavior, distinguish internal range from clipping, and defend your
contract. Next comes aligned summation and its preservation/cancellation tests.

References:
  https://digital-media-study.vercel.app/sampling
  https://wiki.xiph.org/Videos/Digital_Show_and_Tell
