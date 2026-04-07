package clock

import (
	"testing"
	"time"
)

func TestRTPWrapTracker_NoWrapMonotonic(t *testing.T) {
	var wt RTPWrapTracker

	v1 := wt.ExtendedRTP(100)
	if v1 != 100 {
		t.Fatalf("v1=%d want=100", v1)
	}
	v2 := wt.ExtendedRTP(200)
	if v2 != 200 {
		t.Fatalf("v2=%d want=200", v2)
	}
	if wt.wraps != 0 {
		t.Fatalf("unexpected wraps=%d want=0", wt.wraps)
	}
}

func TestRTPWrapTracker_WrapAcrossBoundary(t *testing.T) {
	var wt RTPWrapTracker

	// Simulate near rollover then low value crossing wrap boundary
	v1 := wt.ExtendedRTP(0xFFFFFF00)
	if v1 != 0xFFFFFF00 {
		t.Fatalf("v1=%d want=%d", v1, uint64(0xFFFFFF00))
	}
	v2 := wt.ExtendedRTP(0x00000040)
	// Expect wraps=1 and extended = 2^32 + 0x40
	want := uint64(1<<32) + 0x40
	if v2 != want {
		t.Fatalf("v2=%d want=%d", v2, want)
	}
	if wt.wraps != 1 {
		t.Fatalf("wraps=%d want=1", wt.wraps)
	}
}

func TestRTPWrapTracker_SmallBackstep_NoWrap(t *testing.T) {
	var wt RTPWrapTracker
	_ = wt.ExtendedRTP(5000)
	v := wt.ExtendedRTP(4900)
	if v != 4900 {
		t.Fatalf("extended=%d want=4900 (no wrap on small backstep)", v)
	}
	if wt.wraps != 0 {
		t.Fatalf("wraps=%d want=0", wt.wraps)
	}
}

func TestRTPWrapTracker_TwoWraps(t *testing.T) {
	var wt RTPWrapTracker
	_ = wt.ExtendedRTP(0xFFFFFFF0)
	v2 := wt.ExtendedRTP(0x00000010) // first wrap
	want2 := uint64(1<<32) + 0x10
	if v2 != want2 || wt.wraps != 1 {
		t.Fatalf("after first wrap: v=%d want=%d wraps=%d wantWraps=1", v2, want2, wt.wraps)
	}
	v3 := wt.ExtendedRTP(0xFFFFFFF0) // large forward, no wrap
	want3 := uint64(1<<32) + 0xFFFFFFF0
	if v3 != want3 || wt.wraps != 1 {
		t.Fatalf("forward: v=%d want=%d wraps=%d", v3, want3, wt.wraps)
	}
	v4 := wt.ExtendedRTP(0x00000020) // second wrap
	want4 := uint64(2*(1<<32)) + 0x20
	if v4 != want4 || wt.wraps != 2 {
		t.Fatalf("after second wrap: v=%d want=%d wraps=%d wantWraps=2", v4, want4, wt.wraps)
	}
}

func TestRTPWrapTracker_ThresholdExactlyHalfRange_NoWrap(t *testing.T) {
	var wt RTPWrapTracker
	_ = wt.ExtendedRTP(0x80000000)
	v := wt.ExtendedRTP(0x00000000)
	// last - ts = 2^31; code checks strictly greater than 2^31, so no wrap
	if v != 0 {
		t.Fatalf("extended=%d want=0 (no wrap at threshold)", v)
	}
	if wt.wraps != 0 {
		t.Fatalf("wraps=%d want=0", wt.wraps)
	}
}

func TestStreamClock_AnchorInitializesState(t *testing.T) {
	sc := StreamClock{
		degraded:      true,
		discontinuity: true,
		state:         Uninitialized,
	}

	const (
		rtpTs = uint32(12345)
		ntpTs = 1000.25
		rate  = uint32(90000)
	)

	sc.SetClockRate(rate)
	sc.Anchor(rtpTs, ntpTs)

	if sc.state != Anchored {
		t.Fatalf("state=%v want=%v", sc.state, Anchored)
	}
	if sc.baseRTP != uint64(rtpTs) {
		t.Fatalf("baseRTP=%d want=%d", sc.baseRTP, rtpTs)
	}
	if sc.baseNTP != ntpTs {
		t.Fatalf("baseNTP=%f want=%f", sc.baseNTP, ntpTs)
	}
	if sc.lastNTPOutput != ntpTs {
		t.Fatalf("lastNTPOutput=%f want=%f", sc.lastNTPOutput, ntpTs)
	}
	if sc.wrapTracker.lastRTP != rtpTs || sc.wrapTracker.wraps != 0 {
		t.Fatalf("wrapTracker=%+v want lastRTP=%d wraps=0", sc.wrapTracker, rtpTs)
	}
	if sc.degraded {
		t.Fatal("expected degraded=false after anchor")
	}
	if sc.discontinuity {
		t.Fatal("expected discontinuity=false after anchor")
	}
	if sc.lastSRReceivedAt.IsZero() {
		t.Fatal("expected lastSRReceivedAt to be set")
	}
}

func TestStreamClock_LockSetsLockedState(t *testing.T) {
	sc := StreamClock{state: Anchored}

	sc.Lock()

	if sc.state != Locked {
		t.Fatalf("state=%v want=%v", sc.state, Locked)
	}
}

func TestStreamClock_UpdateClearsDiscontinuityOnStableSR(t *testing.T) {
	sc := StreamClock{
		clockRate:     90000,
		degraded:      true,
		discontinuity: true,
	}
	sc.Anchor(0, 1000.0)

	rtpTs := uint32(90000)
	ntpTs := 1001.0
	sc.Update(rtpTs, ntpTs, 50*time.Millisecond)

	if sc.baseRTP != uint64(rtpTs) {
		t.Fatalf("baseRTP=%d want=%d", sc.baseRTP, rtpTs)
	}
	if sc.baseNTP != ntpTs {
		t.Fatalf("baseNTP=%f want=%f", sc.baseNTP, ntpTs)
	}
	if sc.degraded {
		t.Fatal("expected degraded=false after stable update")
	}
	if sc.discontinuity {
		t.Fatal("expected discontinuity=false after stable update")
	}
}

func TestStreamClock_UpdateMarksDiscontinuityOnLargeJump(t *testing.T) {
	sc := StreamClock{clockRate: 90000}
	sc.Anchor(0, 1000.0)

	rtpTs := uint32(90000)
	ntpTs := 1001.060
	sc.Update(rtpTs, ntpTs, 50*time.Millisecond)

	if sc.baseRTP != uint64(rtpTs) {
		t.Fatalf("baseRTP=%d want=%d", sc.baseRTP, rtpTs)
	}
	if sc.baseNTP != ntpTs {
		t.Fatalf("baseNTP=%f want=%f", sc.baseNTP, ntpTs)
	}
	if !sc.degraded {
		t.Fatal("expected degraded=true after discontinuity")
	}
	if !sc.discontinuity {
		t.Fatal("expected discontinuity=true after discontinuity")
	}
	if sc.lastNTPOutput != ntpTs {
		t.Fatalf("lastNTPOutput=%f want=%f", sc.lastNTPOutput, ntpTs)
	}
}

func TestStreamClock_NtpTimeUsesCurrentAnchor(t *testing.T) {
	sc := StreamClock{clockRate: 90000}
	sc.Anchor(1000, 500.0)

	got := sc.NtpTime(1000 + 9000)
	want := 500.0 + 0.1

	if !almostEqual(got, want, 1e-9) {
		t.Fatalf("NtpTime=%.9f want=%.9f", got, want)
	}
}

func TestStreamClock_DetectDiscontinuityThreshold(t *testing.T) {
	sc := StreamClock{clockRate: 90000}
	sc.Anchor(0, 1000.0)

	ext := uint64(90000)

	if sc.detectDiscontinuity(ext, 1001.040, 50*time.Millisecond) {
		t.Fatal("expected 40ms delta to stay below threshold")
	}
	if !sc.detectDiscontinuity(ext, 1001.060, 50*time.Millisecond) {
		t.Fatal("expected 60ms delta to exceed threshold")
	}
}

func TestStreamClock_ResetDiscontinuityClearsFlags(t *testing.T) {
	sc := StreamClock{
		degraded:      true,
		discontinuity: true,
	}

	sc.resetDiscontinuity()

	if sc.degraded {
		t.Fatal("expected degraded=false after resetDiscontinuity")
	}
	if sc.discontinuity {
		t.Fatal("expected discontinuity=false after resetDiscontinuity")
	}
}
