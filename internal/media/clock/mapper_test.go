package clock

import (
	"math"
	"testing"
	"time"

	"github.com/ssemakov/lkmcu/internal/logging"
	"github.com/ssemakov/lkmcu/internal/metrics"
	"github.com/ssemakov/lkmcu/internal/testutil"
)

func newTestMapper(cfg *ClockMapperConfig) *ClockMapper {
	if cfg == nil {
		cfg = DefaultClockMapperConfig()
	}
	return NewClockMapper(metrics.NewMetrics(), logging.NewNullLogger("test", "debug"), cfg)
}

func almostEqual(a, b, eps float64) bool { return math.Abs(a-b) <= eps }

func TestMapper_RTPWrapExtension(t *testing.T) {
	cm := newTestMapper(nil)
	const (
		ssrc      = uint32(0xdeadbeef)
		clockRate = uint32(90000)
	)
	cm.SetClockRate(ssrc, clockRate)

	baseRTP := uint32(0xFFFFFF00)
	baseNTP := 1000.0
	hi, lo := testutil.ToNTPParts(baseNTP)
	if err := cm.UpdateFromSR(ssrc, baseRTP, hi, lo); err != nil {
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
			expected: baseNTP + float64(0xA0)/float64(clockRate),
		},
		{
			name:     "post-wrap progression",
			rtp:      0x00000040,
			expected: baseNTP + float64(0x140)/float64(clockRate), // 0x40 + wrap from 0xFF00
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, _, err := cm.RTPToNTP(ssrc, tc.rtp)
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
	cm := newTestMapper(nil)
	const (
		ssrc      = uint32(1)
		clockRate = uint32(90000)
	)
	cm.SetClockRate(ssrc, clockRate)

	// First SR
	ntp1 := 1000.0
	rtp1 := uint32(100000)
	hi1, lo1 := testutil.ToNTPParts(ntp1)
	if err := cm.UpdateFromSR(ssrc, rtp1, hi1, lo1); err != nil {
		t.Fatalf("UpdateFromSR #1 err: %v", err)
	}
	s1 := cm.State(ssrc)

	// Out-of-order SR with older NTP should be ignored
	ntp0 := 999.0
	rtp0 := uint32(90000)
	hi0, lo0 := testutil.ToNTPParts(ntp0)
	if err := cm.UpdateFromSR(ssrc, rtp0, hi0, lo0); err != nil {
		t.Fatalf("UpdateFromSR ooo err: %v", err)
	}

	s2 := cm.State(ssrc)
	if s2.BaseNTP != s1.BaseNTP || s2.BaseRTP != s1.BaseRTP {
		t.Fatalf("out-of-order SR changed state: before=%+v after=%+v", s1, s2)
	}
}

func TestMapper_DiscontinuityDetection(t *testing.T) {
	const (
		ssrc      = uint32(2)
		clockRate = uint32(90000)
	)

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
			cm := newTestMapper(nil)
			cm.SetClockRate(ssrc, clockRate)

			// Initial SR (baseline)
			ntp1 := 1000.0
			rtp1 := uint32(0)
			hi1, lo1 := testutil.ToNTPParts(ntp1)
			if err := cm.UpdateFromSR(ssrc, rtp1, hi1, lo1); err != nil {
				t.Fatalf("UpdateFromSR #1 err: %v", err)
			}

			// SR2 at +1s RTP, with NTP offset by delta
			rtp2 := uint32(clockRate * 1)
			predicted := ntp1 + 1.0
			ntp2 := predicted + tt.deltaMs/1000.0
			hi2, lo2 := testutil.ToNTPParts(ntp2)
			if err := cm.UpdateFromSR(ssrc, rtp2, hi2, lo2); err != nil {
				t.Fatalf("UpdateFromSR #2 err: %v", err)
			}
			st := cm.State(ssrc)
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
	cfg := *DefaultClockMapperConfig()
	cfg.SRTimeout = 2 * time.Millisecond
	cm := newTestMapper(&cfg)

	const (
		ssrc      = uint32(3)
		clockRate = uint32(90000)
	)
	cm.SetClockRate(ssrc, clockRate)

	// Initial SR
	ntp1 := 1000.0
	hi1, lo1 := testutil.ToNTPParts(ntp1)
	if err := cm.UpdateFromSR(ssrc, 0, hi1, lo1); err != nil {
		t.Fatalf("UpdateFromSR err: %v", err)
	}

	time.Sleep(3 * time.Millisecond) // exceed SRTimeout

	// Any RTP should trigger timeout degraded
	_, st, err := cm.RTPToNTP(ssrc, 1000)
	if err == nil {
		t.Fatalf("expected SR timeout error, got nil")
	}
	if !st.Degraded {
		t.Fatalf("expected degraded=true after SR timeout")
	}
}

func TestMapper_SRTimeoutMeasuredFromLatestAcceptedSR(t *testing.T) {
	cfg := *DefaultClockMapperConfig()
	cfg.SRTimeout = 20 * time.Millisecond
	cm := newTestMapper(&cfg)

	const (
		ssrc      = uint32(33)
		clockRate = uint32(90000)
	)
	cm.SetClockRate(ssrc, clockRate)

	// First SR anchors the clock.
	ntp1 := 1000.0
	rtp1 := uint32(0)
	hi1, lo1 := testutil.ToNTPParts(ntp1)
	if err := cm.UpdateFromSR(ssrc, rtp1, hi1, lo1); err != nil {
		t.Fatalf("UpdateFromSR #1 err: %v", err)
	}
	st1 := cm.State(ssrc)
	if st1.LastSRReceivedAt.IsZero() {
		t.Fatal("expected first SR to set LastSRReceivedAt")
	}

	time.Sleep(10 * time.Millisecond)

	// Second stable SR should refresh the SR age origin.
	rtp2 := uint32(clockRate)
	ntp2 := ntp1 + 1.0
	hi2, lo2 := testutil.ToNTPParts(ntp2)
	if err := cm.UpdateFromSR(ssrc, rtp2, hi2, lo2); err != nil {
		t.Fatalf("UpdateFromSR #2 err: %v", err)
	}
	st2 := cm.State(ssrc)
	if !st2.LastSRReceivedAt.After(st1.LastSRReceivedAt) {
		t.Fatalf("expected LastSRReceivedAt to move forward, before=%v after=%v", st1.LastSRReceivedAt, st2.LastSRReceivedAt)
	}

	// This exceeds the age from SR1, but should still be below the timeout from SR2.
	time.Sleep(12 * time.Millisecond)

	_, st, err := cm.RTPToNTP(ssrc, rtp2+1000)
	if err != nil {
		t.Fatalf("unexpected timeout measured from SR1 instead of SR2: %v", err)
	}
	if st.Degraded {
		t.Fatalf("unexpected degraded=true before SR2-based timeout, state=%+v", st)
	}

	// Now exceed the timeout relative to SR2 and expect degradation.
	time.Sleep(12 * time.Millisecond)

	_, st, err = cm.RTPToNTP(ssrc, rtp2+2000)
	if err == nil {
		t.Fatal("expected SR timeout error after exceeding SR2-based timeout")
	}
	if !st.Degraded {
		t.Fatalf("expected degraded=true after SR2-based timeout, state=%+v", st)
	}
}

func TestMapper_MonotonicGuard_SmallBackstep(t *testing.T) {
	cm := newTestMapper(nil)
	const (
		ssrc      = uint32(4)
		clockRate = uint32(90000)
	)
	cm.SetClockRate(ssrc, clockRate)

	// Baseline SR1
	baseNTP := 1000.0
	hi1, lo1 := testutil.ToNTPParts(baseNTP)
	if err := cm.UpdateFromSR(ssrc, 0, hi1, lo1); err != nil {
		t.Fatalf("UpdateFromSR #1 err: %v", err)
	}
	// SR2 consistent at +1s to clear degraded
	r2 := uint32(clockRate * 1)
	hi2, lo2 := testutil.ToNTPParts(baseNTP + 1.0)
	if err := cm.UpdateFromSR(ssrc, r2, hi2, lo2); err != nil {
		t.Fatalf("UpdateFromSR #2 err: %v", err)
	}
	// Produce last output at r3
	r3 := uint32(200000)
	got1, _, err := cm.RTPToNTP(ssrc, r3)
	if err != nil {
		t.Fatalf("RTPToNTP #1 err: %v", err)
	}
	// SR3: small backstep of 5ms at same RTP base (no discontinuity)
	hi3, lo3 := testutil.ToNTPParts(got1 - 0.005)
	if err := cm.UpdateFromSR(ssrc, r3, hi3, lo3); err != nil {
		t.Fatalf("UpdateFromSR #3 err: %v", err)
	}
	// Next packet slightly after SR3 base should backstep < threshold and clamp to last+ smoothing
	got2, st, err := cm.RTPToNTP(ssrc, r3+1)
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

func TestMapper_MonotonicGuard_LargeBackstep(t *testing.T) {
	cm := newTestMapper(nil)
	const (
		ssrc      = uint32(5)
		clockRate = uint32(90000)
	)
	cm.SetClockRate(ssrc, clockRate)

	// Baseline SR1
	baseNTP := 1000.0
	hi1, lo1 := testutil.ToNTPParts(baseNTP)
	if err := cm.UpdateFromSR(ssrc, 0, hi1, lo1); err != nil {
		t.Fatalf("UpdateFromSR #1 err: %v", err)
	}
	// SR2 consistent to clear degraded
	r2 := uint32(clockRate * 1)
	hi2, lo2 := testutil.ToNTPParts(baseNTP + 1.0)
	if err := cm.UpdateFromSR(ssrc, r2, hi2, lo2); err != nil {
		t.Fatalf("UpdateFromSR #2 err: %v", err)
	}
	// Produce last output at r3 (big timestamp)
	r3 := uint32(200000)
	last, _, err := cm.RTPToNTP(ssrc, r3)
	if err != nil {
		t.Fatalf("RTPToNTP err: %v", err)
	}
	// SR3: large backstep of 100ms at same base rtp (should not trigger SR discontinuity)
	hi3, lo3 := testutil.ToNTPParts(last - 0.100)
	if err := cm.UpdateFromSR(ssrc, r3, hi3, lo3); err != nil {
		t.Fatalf("UpdateFromSR #3 err: %v", err)
	}
	// Next RTP predicted < last; large backstep should mark degraded + discontinuity.
	_, st, _ := cm.RTPToNTP(ssrc, r3+1)
	if !st.Degraded {
		t.Fatalf("expected degraded=true for large backstep, got state=%+v", st)
	}
}
