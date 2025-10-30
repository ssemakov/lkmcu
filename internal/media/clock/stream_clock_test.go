package clock

import "testing"

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

