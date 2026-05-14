package clock

import (
	"math"
	"testing"
	"time"

	"github.com/ssemakov/lkmcu/internal/logging"
	"github.com/ssemakov/lkmcu/internal/metrics"
)

type testSR struct {
	ssrc      uint32
	clockRate uint32
}

// newTestMapper creates a ClockMapper fake, if cfg is nil it will use DefaultClockMapperConfig
func newTestMapper(cfg *ClockMapperConfig, sr *testSR) *ClockMapper {
	if cfg == nil {
		cfg = DefaultClockMapperConfig()
	}

	cm := NewClockMapper(metrics.NewMetrics(), logging.NewNullLogger("test", "debug"), cfg)
	if sr != nil {
		cm.SetClockRate(sr.ssrc, sr.clockRate)
	}
	return cm
}

func almostEqual(a, b, eps float64) bool { return math.Abs(a-b) <= eps }

func TestMapper_RTPWrapExtension(t *testing.T) {
	sr := testSR{
		ssrc:      uint32(0xdeadbeef),
		clockRate: uint32(90000),
	}

	cm := newTestMapper(nil, &sr)

	baseRTP := uint32(0xFFFFFF00)
	baseNTP := 1000.0
	hi, lo := ToNTPParts(baseNTP)
	if err := cm.UpdateFromSR(sr.ssrc, baseRTP, hi, lo); err != nil {
		t.Fatalf("UpdateFromSR(base) err: %v", err)
	}

	cases := []struct {
		name     string
		rtp      uint32
		expected float64
	}{
		{
			name:     "pre-wrap progression",
			rtp:      0xFFFFFFA0,
			expected: baseNTP + float64(0xA0)/float64(sr.clockRate),
		},
		{
			name:     "post-wrap progression",
			rtp:      0x00000040,
			expected: baseNTP + float64(0x140)/float64(sr.clockRate), // 0x40 + wrap from 0xFF00
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, _, err := cm.RTPToNTP(sr.ssrc, tc.rtp)
			if err != nil {
				t.Fatalf("RTPToNTP err: %v", err)
			}
			if !almostEqual(got, tc.expected, 1e-6) {
				t.Fatalf("got %.9f, want %.9f", got, tc.expected)
			}
		})
	}
}

func TestMapper_OutOfOrderSRSuppression(t *testing.T) {
	sr := testSR{
		ssrc:      uint32(0xdeadbeef),
		clockRate: uint32(90000),
	}

	cm := newTestMapper(nil, &sr)

	// First SR
	ntp1 := 1000.0
	rtp1 := uint32(100000)
	hi1, lo1 := ToNTPParts(ntp1)
	if err := cm.UpdateFromSR(sr.ssrc, rtp1, hi1, lo1); err != nil {
		t.Fatalf("UpdateFromSR #1 err: %v", err)
	}
	s1 := cm.State(sr.ssrc)

	// Out-of-order SR with older NTP should be ignored
	ntp0 := 999.0
	rtp0 := uint32(90000)
	hi0, lo0 := ToNTPParts(ntp0)
	if err := cm.UpdateFromSR(sr.ssrc, rtp0, hi0, lo0); err != nil {
		t.Fatalf("UpdateFromSR ooo err: %v", err)
	}

	s2 := cm.State(sr.ssrc)
	if s2.BaseNTP != s1.BaseNTP || s2.BaseRTP != s1.BaseRTP {
		t.Fatalf("out-of-order SR changed state: before=%+v after=%+v", s1, s2)
	}
}

func TestMapper_DiscontinuityDetection(t *testing.T) {
	sr := testSR{
		ssrc:      uint32(0xdeadbeef),
		clockRate: uint32(90000),
	}

	type tc struct {
		name           string
		deltaMs        float64
		expectDisc     bool
		expectDegraded bool
	}
	tests := []tc{
		{name: "below-threshold", deltaMs: 40, expectDisc: false, expectDegraded: false},
		{name: "above-threshold", deltaMs: 60, expectDisc: true, expectDegraded: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm := newTestMapper(nil, &sr)

			// Initial SR (baseline)
			ntp1 := 1000.0
			rtp1 := uint32(0)
			hi1, lo1 := ToNTPParts(ntp1)
			if err := cm.UpdateFromSR(sr.ssrc, rtp1, hi1, lo1); err != nil {
				t.Fatalf("UpdateFromSR #1 err: %v", err)
			}

			// SR2 at +1s RTP, with NTP offset by delta
			rtp2 := uint32(sr.clockRate * 1)
			predicted := ntp1 + 1.0
			ntp2 := predicted + tt.deltaMs/1000.0
			hi2, lo2 := ToNTPParts(ntp2)
			if err := cm.UpdateFromSR(sr.ssrc, rtp2, hi2, lo2); err != nil {
				t.Fatalf("UpdateFromSR #2 err: %v", err)
			}
			st := cm.State(sr.ssrc)
			if st.Discontinuity != tt.expectDisc {
				t.Fatalf("Discontinuity=%v want %v (deltaMs=%.1f)", st.Discontinuity, tt.expectDisc, tt.deltaMs)
			}
			if st.Degraded != tt.expectDegraded {
				t.Fatalf("Degraded=%v want %v (deltaMs=%.1f)", st.Degraded, tt.expectDegraded, tt.deltaMs)
			}
		})
	}
}

func TestMapper_SRTimeoutDegraded(t *testing.T) {
	sr := testSR{
		ssrc:      uint32(0xdeadbeef),
		clockRate: uint32(90000),
	}

	cfg := *DefaultClockMapperConfig()
	cfg.SRTimeout = 2 * time.Millisecond
	cm := newTestMapper(&cfg, &sr)

	// Initial SR
	ntp1 := 1000.0
	hi1, lo1 := ToNTPParts(ntp1)
	if err := cm.UpdateFromSR(sr.ssrc, 0, hi1, lo1); err != nil {
		t.Fatalf("UpdateFromSR err: %v", err)
	}

	time.Sleep(3 * time.Millisecond) // exceed SRTimeout

	// Any RTP should trigger timeout degraded
	_, st, err := cm.RTPToNTP(sr.ssrc, 1000)
	if err == nil {
		t.Fatalf("expected SR timeout error, got nil")
	}
	if !st.Degraded {
		t.Fatalf("expected degraded=true after SR timeout")
	}
}

func TestMapper_SRTimeoutMeasuredFromLatestAcceptedSR(t *testing.T) {
	sr := testSR{
		ssrc:      uint32(0xdeadbeef),
		clockRate: uint32(90000),
	}

	cfg := *DefaultClockMapperConfig()
	cfg.SRTimeout = 20 * time.Millisecond
	cm := newTestMapper(&cfg, &sr)

	// First SR anchors the clock.
	ntp1 := 1000.0
	rtp1 := uint32(0)
	hi1, lo1 := ToNTPParts(ntp1)
	if err := cm.UpdateFromSR(sr.ssrc, rtp1, hi1, lo1); err != nil {
		t.Fatalf("UpdateFromSR #1 err: %v", err)
	}
	st1 := cm.State(sr.ssrc)
	if st1.LastSRReceivedAt.IsZero() {
		t.Fatal("expected first SR to set LastSRReceivedAt")
	}

	time.Sleep(10 * time.Millisecond)

	// Second stable SR should refresh the SR age origin.
	rtp2 := uint32(sr.clockRate)
	ntp2 := ntp1 + 1.0
	hi2, lo2 := ToNTPParts(ntp2)
	if err := cm.UpdateFromSR(sr.ssrc, rtp2, hi2, lo2); err != nil {
		t.Fatalf("UpdateFromSR #2 err: %v", err)
	}
	st2 := cm.State(sr.ssrc)
	if !st2.LastSRReceivedAt.After(st1.LastSRReceivedAt) {
		t.Fatalf("expected LastSRReceivedAt to move forward, before=%v after=%v", st1.LastSRReceivedAt, st2.LastSRReceivedAt)
	}

	// This exceeds the age from SR1, but should still be below the timeout from SR2.
	time.Sleep(12 * time.Millisecond)

	_, st, err := cm.RTPToNTP(sr.ssrc, rtp2+1000)
	if err != nil {
		t.Fatalf("unexpected timeout measured from SR1 instead of SR2: %v", err)
	}
	if st.Degraded {
		t.Fatalf("unexpected degraded=true before SR2-based timeout, state=%+v", st)
	}

	// Now exceed the timeout relative to SR2 and expect degradation.
	time.Sleep(12 * time.Millisecond)

	_, st, err = cm.RTPToNTP(sr.ssrc, rtp2+2000)
	if err == nil {
		t.Fatal("expected SR timeout error after exceeding SR2-based timeout")
	}
	if !st.Degraded {
		t.Fatalf("expected degraded=true after SR2-based timeout, state=%+v", st)
	}
}

func TestMapper_MonotonicGuard_SmallBackstep(t *testing.T) {
	sr := testSR{
		ssrc:      uint32(0xdeadbeef),
		clockRate: uint32(90000),
	}

	cm := newTestMapper(nil, &sr)

	// Baseline SR1
	baseNTP := 1000.0
	hi1, lo1 := ToNTPParts(baseNTP)
	if err := cm.UpdateFromSR(sr.ssrc, 0, hi1, lo1); err != nil {
		t.Fatalf("UpdateFromSR #1 err: %v", err)
	}
	// SR2 consistent at +1s to clear degraded
	r2 := uint32(sr.clockRate * 1)
	hi2, lo2 := ToNTPParts(baseNTP + 1.0)
	if err := cm.UpdateFromSR(sr.ssrc, r2, hi2, lo2); err != nil {
		t.Fatalf("UpdateFromSR #2 err: %v", err)
	}
	// Produce last output at r3
	r3 := uint32(200000)
	got1, _, err := cm.RTPToNTP(sr.ssrc, r3)
	if err != nil {
		t.Fatalf("RTPToNTP #1 err: %v", err)
	}
	// SR3: small backstep of 5ms at same RTP base (no discontinuity)
	hi3, lo3 := ToNTPParts(got1 - 0.005)
	if err := cm.UpdateFromSR(sr.ssrc, r3, hi3, lo3); err != nil {
		t.Fatalf("UpdateFromSR #3 err: %v", err)
	}
	// Next packet slightly after SR3 base should backstep < threshold and clamp to last+ smoothing
	got2, st, err := cm.RTPToNTP(sr.ssrc, r3+1)
	if err != nil {
		t.Fatalf("RTPToNTP #2 err: %v", err)
	}
	if st.Degraded {
		t.Fatalf("unexpected degraded=true for small backstep clamp")
	}
	want := got1 + DefaultClockMapperConfig().ClampSmoothing.Seconds()
	if !almostEqual(got2, want, 1e-6) {
		t.Fatalf("clamped ntp=%.9f, want %.9f (last=%.9f)", got2, want, got1)
	}
}

func TestMapper_UpdateFromSR_TransitionUnitializedAnchoredLocked(t *testing.T) {
	sr := testSR{
		ssrc:      uint32(0xdeadbeef),
		clockRate: uint32(90000),
	}

	cm := newTestMapper(nil, &sr)
	state := cm.State(sr.ssrc)
	if state.Clock != Uninitialized {
		t.Fatalf("initial Clock State=%v, want %v", state.Clock, Uninitialized)
	}

	if state.HasSenderReport {
		t.Fatalf("initial HasSenderReport=true, want false")
	}

	ntp1 := 1000.0
	rtp1 := uint32(0)
	hi1, lo1 := ToNTPParts(ntp1)

	if err := cm.UpdateFromSR(sr.ssrc, rtp1, hi1, lo1); err != nil {
		t.Fatalf("first UpdateFromSR error = %v", err)
	}

	state = cm.State(sr.ssrc)
	if state.Clock != Anchored {
		t.Fatalf("after first SR Clock=%v want %v", state.Clock, Anchored)
	}
	if !state.HasSenderReport {
		t.Fatal("expected sender report after first SR")
	}
	if state.BaseRTP != uint64(rtp1) {
		t.Fatalf("BaseRTP=%d want %d", state.BaseRTP, rtp1)
	}
	if state.BaseNTP != ntp1 {
		t.Fatalf("BaseNTP=%f want %f", state.BaseNTP, ntp1)
	}

	ntp2 := 1001.0
	rtp2 := uint32(sr.clockRate)
	hi2, lo2 := ToNTPParts(ntp2)

	if err := cm.UpdateFromSR(sr.ssrc, rtp2, hi2, lo2); err != nil {
		t.Fatalf("UpdateFromSR #2 error = %v", err)
	}

	state = cm.State(sr.ssrc)
	if state.Clock != Locked {
		t.Fatalf("after first SR Clock=%v want %v", state.Clock, Locked)
	}
	if !state.HasSenderReport {
		t.Fatal("expected sender report after first SR")
	}
	if state.BaseRTP != uint64(rtp2) {
		t.Fatalf("BaseRTP=%d want %d", state.BaseRTP, rtp2)
	}
	if state.BaseNTP != ntp2 {
		t.Fatalf("BaseNTP=%f want %f", state.BaseNTP, ntp2)
	}
}

func TestMapper_MonotonicGuard_LargeBackstep(t *testing.T) {
	sr := testSR{
		ssrc:      uint32(0xdeadbeef),
		clockRate: uint32(90000),
	}

	cm := newTestMapper(nil, &sr)

	// Baseline SR1
	baseNTP := 1000.0
	hi1, lo1 := ToNTPParts(baseNTP)
	if err := cm.UpdateFromSR(sr.ssrc, 0, hi1, lo1); err != nil {
		t.Fatalf("UpdateFromSR #1 err: %v", err)
	}
	// SR2 consistent to clear degraded
	r2 := uint32(sr.clockRate * 1)
	hi2, lo2 := ToNTPParts(baseNTP + 1.0)
	if err := cm.UpdateFromSR(sr.ssrc, r2, hi2, lo2); err != nil {
		t.Fatalf("UpdateFromSR #2 err: %v", err)
	}
	// Produce last output at r3 (big timestamp)
	r3 := uint32(200000)
	last, _, err := cm.RTPToNTP(sr.ssrc, r3)
	if err != nil {
		t.Fatalf("RTPToNTP err: %v", err)
	}
	// SR3: large backstep of 100ms at same base rtp (should not trigger SR discontinuity)
	hi3, lo3 := ToNTPParts(last - 0.100)
	if err := cm.UpdateFromSR(sr.ssrc, r3, hi3, lo3); err != nil {
		t.Fatalf("UpdateFromSR #3 err: %v", err)
	}
	// Next RTP predicted < last; large backstep should mark degraded + discontinuity.
	_, st, _ := cm.RTPToNTP(sr.ssrc, r3+1)
	if !st.Degraded {
		t.Fatalf("expected degraded=true for large backstep, got state=%+v", st)
	}
}

func TestMapper_ForEach_EmptyMapper(t *testing.T) {
	cm := newTestMapper(nil, nil)
	calls := 0
	f := func(ssrc uint32, state MapperState) bool { calls++; return true }
	cm.ForEach(f)

	if calls != 0 {
		t.Fatalf("expected zero calls, got %v", calls)
	}
}

func TestMapper_ForEach_IncludesUninitializedStream(t *testing.T) {
	sr := testSR{
		ssrc:      uint32(0xdeadbeef),
		clockRate: uint32(90000),
	}

	cm := newTestMapper(nil, &sr)

	f := func(ssrc uint32, state MapperState) bool {
		if state.Clock != Uninitialized {
			t.Fatalf("expected Clock Uninitialized, got %s", ClockStateString(state.Clock))
		}

		if state.HasSenderReport {
			t.Fatalf("expected HasSenderReport to be false, got true")
		}

		if state.ClockRate != sr.clockRate {
			t.Fatalf("expected ClockRate %v, got %v", state.ClockRate, sr.clockRate)
		}

		if state.LastSRReceivedAt.IsZero() != true {
			t.Fatalf("expected LastSRRecievedAt to be zero, got %v", state.LastSRReceivedAt)
		}
		return true
	}
	cm.ForEach(f)
}

func TestMapper_ForEach_ReportsAllStreams(t *testing.T) {
	const clockRate = uint32(90000)

	uninitSR := testSR{ssrc: 1, clockRate: clockRate}
	anchoredSR := testSR{ssrc: 2, clockRate: clockRate}
	lockedSR := testSR{ssrc: 3, clockRate: clockRate}

	cm := newTestMapper(nil, nil)
	cm.SetClockRate(uninitSR.ssrc, uninitSR.clockRate)
	cm.SetClockRate(anchoredSR.ssrc, anchoredSR.clockRate)
	cm.SetClockRate(lockedSR.ssrc, lockedSR.clockRate)

	anchoredNTP := 1000.0
	anchoredRTP := uint32(0)
	anchoredHi, anchoredLo := ToNTPParts(anchoredNTP)
	if err := cm.UpdateFromSR(anchoredSR.ssrc, anchoredRTP, anchoredHi, anchoredLo); err != nil {
		t.Fatalf("UpdateFromSR anchored stream error = %v", err)
	}

	lockedNTP1 := 2000.0
	lockedRTP1 := uint32(0)
	lockedHi1, lockedLo1 := ToNTPParts(lockedNTP1)
	if err := cm.UpdateFromSR(lockedSR.ssrc, lockedRTP1, lockedHi1, lockedLo1); err != nil {
		t.Fatalf("UpdateFromSR locked stream #1 error = %v", err)
	}

	lockedNTP2 := 2001.0
	lockedRTP2 := uint32(clockRate)
	lockedHi2, lockedLo2 := ToNTPParts(lockedNTP2)
	if err := cm.UpdateFromSR(lockedSR.ssrc, lockedRTP2, lockedHi2, lockedLo2); err != nil {
		t.Fatalf("UpdateFromSR locked stream #2 error = %v", err)
	}

	got := make(map[uint32]MapperState)
	cm.ForEach(func(ssrc uint32, state MapperState) bool {
		got[ssrc] = state
		return true
	})

	if len(got) != 3 {
		t.Fatalf("ForEach returned %d streams, want 3: %+v", len(got), got)
	}

	uninitState := got[uninitSR.ssrc]
	if uninitState.Clock != Uninitialized {
		t.Fatalf("uninitialized Clock=%s, want %s", ClockStateString(uninitState.Clock), ClockStateString(Uninitialized))
	}
	if uninitState.HasSenderReport {
		t.Fatal("uninitialized HasSenderReport=true, want false")
	}

	anchoredState := got[anchoredSR.ssrc]
	if anchoredState.Clock != Anchored {
		t.Fatalf("anchored Clock=%s, want %s", ClockStateString(anchoredState.Clock), ClockStateString(Anchored))
	}
	if !anchoredState.HasSenderReport {
		t.Fatal("anchored HasSenderReport=false, want true")
	}
	if anchoredState.BaseRTP != uint64(anchoredRTP) {
		t.Fatalf("anchored BaseRTP=%d, want %d", anchoredState.BaseRTP, anchoredRTP)
	}
	if anchoredState.BaseNTP != anchoredNTP {
		t.Fatalf("anchored BaseNTP=%f, want %f", anchoredState.BaseNTP, anchoredNTP)
	}

	lockedState := got[lockedSR.ssrc]
	if lockedState.Clock != Locked {
		t.Fatalf("locked Clock=%s, want %s", ClockStateString(lockedState.Clock), ClockStateString(Locked))
	}
	if !lockedState.HasSenderReport {
		t.Fatal("locked HasSenderReport=false, want true")
	}
	if lockedState.BaseRTP != uint64(lockedRTP2) {
		t.Fatalf("locked BaseRTP=%d, want %d", lockedState.BaseRTP, lockedRTP2)
	}
	if lockedState.BaseNTP != lockedNTP2 {
		t.Fatalf("locked BaseNTP=%f, want %f", lockedState.BaseNTP, lockedNTP2)
	}
	if lockedState.Degraded || lockedState.Discontinuity {
		t.Fatalf("locked stream should be stable, got degraded=%v discontinuity=%v", lockedState.Degraded, lockedState.Discontinuity)
	}
}

func TestMapper_ForEach_StopsWhenCallbackReturnsFalse(t *testing.T) {
	cm := newTestMapper(nil, nil)
	cm.SetClockRate(1, 90000)
	cm.SetClockRate(2, 90000)
	cm.SetClockRate(3, 90000)

	calls := 0
	cm.ForEach(func(uint32, MapperState) bool {
		calls++
		return false
	})

	if calls != 1 {
		t.Fatalf("ForEach callback calls=%d, want 1", calls)
	}
}
