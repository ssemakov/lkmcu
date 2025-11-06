package clock

import (
	"testing"
	"time"

	"github.com/ssemakov/lkmcu/internal/logging"
	"github.com/ssemakov/lkmcu/internal/metrics"
)

func TestDrift_NewDriftController(t *testing.T) {
	dc := NewDriftController(metrics.NewMetrics(), logging.NewNullLogger("test", "debug"))

	if dc == nil {
		t.Errorf("Expected non-nil DriftController")
		return
	}

	defaultCfg := DefaultDriftConfig()
	if dc.cfg != defaultCfg {
		t.Errorf("Expected default config to be set")
		return
	}
}

func TestDrift_Reset(t *testing.T) {
	dc := NewDriftController(metrics.NewMetrics(), logging.NewNullLogger("test", "debug"))
	const ssrc = uint32(0x12345678)

	const testingStartTimeUnix = int64(1_600_000_000)
	testingStartTime := time.Unix(testingStartTimeUnix, 0)

	// Observe some packets to create state
	dc.Observe(ssrc, testingStartTime, testingStartTime.Add(10*time.Millisecond))
	dc.Observe(ssrc, testingStartTime.Add(20*time.Millisecond), testingStartTime.Add(30*time.Millisecond))

	// Reset state
	dc.Reset(ssrc)

	// Observe again and ensure state is re-initialized
	assessment := dc.Observe(ssrc, testingStartTime.Add(40*time.Millisecond), testingStartTime.Add(50*time.Millisecond))
	if assessment.currentPpm != 0 || assessment.correctionRecommendation != "NoCorrection" {
		t.Errorf("Expected fresh state after reset, got assessment: %+v", assessment)
	}
}

func TestDrift_PruneExpired(t *testing.T) {
	dc := NewDriftController(metrics.NewMetrics(), logging.NewNullLogger("test", "debug"))
	const ssrc = uint32(0x12345678)

	const testingStartTimeUnix = int64(1_761_955_200)
	testingStartTime := time.Unix(testingStartTimeUnix, 0)

	// Observe some packets to create state
	dc.Observe(ssrc, testingStartTime, testingStartTime.Add(10*time.Millisecond))
	dc.Observe(ssrc, testingStartTime.Add(20*time.Millisecond), testingStartTime.Add(30*time.Millisecond))

	// Advance time beyond expiration threshold
	timeAfterExpiration := testingStartTime.Add(dc.cfg.WindowDuration + 1*time.Second)

	// Prune expired state
	dc.PruneExpired(timeAfterExpiration)

	// Observe again and ensure state is re-initialized
	assessment := dc.Observe(ssrc, timeAfterExpiration, timeAfterExpiration.Add(10*time.Millisecond))
	if assessment.currentPpm != 0 || assessment.correctionRecommendation != "NoCorrection" {
		t.Errorf("Expected fresh state after pruning expired, got assessment: %+v", assessment)
	}
}

func TestDrift_ObserveZeroDeltaTimeSkipsEWMA(t *testing.T) {
	dc := NewDriftController(metrics.NewMetrics(), logging.NewNullLogger("test", "debug"))
	const ssrc = uint32(0x12345678)

	const testingStartTimeUnix = int64(1_761_955_200)
	testingStartTime := time.Unix(testingStartTimeUnix, 0)

	// First observation to initialize state
	dc.Observe(ssrc, testingStartTime, testingStartTime.Add(10*time.Millisecond))

	// Second observation with zero delta time
	assessment := dc.Observe(ssrc, testingStartTime, testingStartTime.Add(10*time.Millisecond))

	if assessment.currentPpm != 0 || assessment.correctionRecommendation != "NoCorrection" {
		t.Errorf("Expected no drift update on zero delta time, got assessment: %+v", assessment)
	}
}

func TestDrift_ObserveMaintainsEWMAHistory(t *testing.T) {
	dc := NewDriftController(metrics.NewMetrics(), logging.NewNullLogger("test", "debug"))
	const ssrc = uint32(0x12345678)

	const testingStartTimeUnix = int64(1_761_955_200)
	testingStartTime := time.Unix(testingStartTimeUnix, 0)

	// Feed in multiple observations with known drift
	for i := 0; i < 50; i++ {
		expected := testingStartTime.Add(time.Duration(i) * 20 * time.Millisecond)
		arrival := expected.Add(time.Duration(i) * time.Microsecond) // Simulate slight drift
		dc.Observe(ssrc, expected, arrival)
	}

	assessment := dc.Observe(ssrc, testingStartTime.Add(1*time.Second), testingStartTime.Add(1*time.Second).Add(60*time.Millisecond))

	if assessment.correctionRecommendation != "NoCorrection" {
		t.Errorf("Expected no correction recommendation yet, got assessment: %+v", assessment)
	}
}

func TestDrift_AssessPPMHysteresis(t *testing.T) {
	tests := []struct {
		name                   string
		getSustainDuration     func(cfg DriftConfig) time.Duration
		getPPM                 func(cfg DriftConfig) float64
		expectedRecommendation string
	}{
		{
			name:                   "WarnPPM triggers AdjustSlightly after sustained drift",
			getSustainDuration:     func(cfg DriftConfig) time.Duration { return cfg.SustainWarn + 5*time.Second },
			getPPM:                 func(cfg DriftConfig) float64 { return cfg.WarnPPM + 1 },
			expectedRecommendation: "AdjustSlightly",
		},
		{
			name:                   "CritPPM triggers RequestResync after sustained drift",
			getSustainDuration:     func(cfg DriftConfig) time.Duration { return cfg.SustainCrit + 5*time.Second },
			getPPM:                 func(cfg DriftConfig) float64 { return cfg.CritPPM + 1 },
			expectedRecommendation: "RequestResync",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dc := NewDriftController(metrics.NewMetrics(), logging.NewNullLogger("test", "debug"))
			const ssrc = uint32(0x12345678)
			const testingStartTimeUnix = int64(1_761_955_200)
			testingStartTime := time.Unix(testingStartTimeUnix, 0)

			sustainDuration := tt.getSustainDuration(dc.cfg)
			interval := 20 * time.Millisecond // arrival interval
			steps := int(sustainDuration / interval)

			// Each successive sample arrives slightly later than previous one so the EWMA slope remains above CritPPM slightly
			// and sustains for the required duration
			perStepDrift := time.Duration(float64(interval) * tt.getPPM(dc.cfg) / 1e6)

			for i := 0; i < steps; i++ {
				expected := testingStartTime.Add(time.Duration(i) * interval)
				arrival := expected.Add(time.Duration(i) * perStepDrift)
				dc.Observe(ssrc, expected, arrival)
			}

			assessment := dc.Observe(
				ssrc,
				testingStartTime.Add(sustainDuration),
				testingStartTime.Add(sustainDuration).Add(60*time.Millisecond),
			)

			if assessment.correctionRecommendation != correctionAction(tt.expectedRecommendation) {
				t.Errorf("Expected %s recommendation after sustained drift, got assessment: %+v", tt.expectedRecommendation, assessment)
			}
			if !assessment.hysteresisApplied {
				t.Errorf("Expected hysteresis to be applied after sustained drift, got assessment: %+v", assessment)
			}
		})
	}
}

func TestDrift_AssessPPMResetClearsHysteresis(t *testing.T) {
	dc := NewDriftController(metrics.NewMetrics(), logging.NewNullLogger("test", "debug"))
	const ssrc = uint32(0x12345678)

	const testingStartTimeUnix = int64(1_761_955_200)
	testingStartTime := time.Unix(testingStartTimeUnix, 0)

	// First, sustain high drift to trigger a recommendation
	sustainDuration := dc.cfg.SustainCrit + 5*time.Second
	interval := 20 * time.Millisecond // arrival interval
	steps := int(sustainDuration / interval)
	perStepDrift := time.Duration(float64(interval) * (dc.cfg.CritPPM + 1) / 1e6)

	var (
		assessment   DriftAssessment
		lastExpected time.Time
		lastArrival  time.Time
	)

	for i := 0; i < steps; i++ {
		expected := testingStartTime.Add(time.Duration(i) * interval)
		arrival := expected.Add(time.Duration(i) * perStepDrift)
		assessment = dc.Observe(ssrc, expected, arrival)
		lastExpected = expected
		lastArrival = arrival
	}

	if assessment.correctionRecommendation != RequestResync {
		t.Errorf("Expected RequestResync recommendation after sustained drift, got assessment: %+v", assessment)
	}

	// Next, feed in low-drift samples to reset hysteresis timers
	lowDriftSteps := 20
	currentExpected := lastExpected
	lowDriftErr := lastArrival.Sub(lastExpected)

	for i := 0; i < lowDriftSteps; i++ {
		currentExpected = currentExpected.Add(interval)
		arrival := currentExpected.Add(lowDriftErr)
		assessment = dc.Observe(ssrc, currentExpected, arrival)
	}

	if assessment.hysteresisApplied {
		t.Errorf("Expected hysteresis to be cleared after low drift period, got assessment: %+v", assessment)
	}
}

func TestDrift_PruneExpiredRetainsRecentEntries(t *testing.T) {
	dc := NewDriftController(metrics.NewMetrics(), logging.NewNullLogger("test", "debug"))
	const ssrc = uint32(0x12345678)

	const testingStartTimeUnix = int64(1_761_955_200)
	testingStartTime := time.Unix(testingStartTimeUnix, 0)

	// Observe some packets to create state
	dc.Observe(ssrc, testingStartTime, testingStartTime.Add(10*time.Millisecond))
	// Capture pre-prune state reference
	pre := dc.dm[ssrc]
	dc.Observe(ssrc, testingStartTime.Add(20*time.Millisecond), testingStartTime.Add(30*time.Millisecond))

	// Advance time but stay within expiration threshold
	timeWithinExpiration := testingStartTime.Add(dc.cfg.WindowDuration - 1*time.Second)

	// Prune expired state
	dc.PruneExpired(timeWithinExpiration)

	if dc.dm[ssrc] != pre {
		t.Errorf("Expected SSRC state to exist")
		return
	}

	// Observe again and ensure state is retained
	assessment := dc.Observe(ssrc, timeWithinExpiration, timeWithinExpiration.Add(15*time.Millisecond))
	if assessment.currentPpm == 0 && assessment.correctionRecommendation == "NoCorrection" {
		t.Errorf("Expected retained state after pruning non-expired, got assessment: %+v", assessment)
	}
}

func TestDrift_ObserveConcurrentSSRCs(t *testing.T) {
	dc := NewDriftController(metrics.NewMetrics(), logging.NewNullLogger("test", "debug"))
	const ssrc1 = uint32(0x12345678)
	const ssrc2 = uint32(0x87654321)

	const testingStartTimeUnix = int64(1_761_955_200)
	testingStartTime := time.Unix(testingStartTimeUnix, 0)

	// Observe on SSRC1
	dc.Observe(ssrc1, testingStartTime, testingStartTime.Add(10*time.Millisecond))

	// Observe on SSRC2 concurrently
	done := make(chan struct{})
	go func() {
		dc.Observe(ssrc2, testingStartTime, testingStartTime.Add(15*time.Millisecond))
		close(done)
	}()

	<-done

	dc.Observe(ssrc1, testingStartTime.Add(20*time.Millisecond), testingStartTime.Add(30*time.Millisecond))
	dc.Observe(ssrc2, testingStartTime.Add(20*time.Millisecond), testingStartTime.Add(35*time.Millisecond))

	if dc.dm[ssrc1] == nil || dc.dm[ssrc2] == nil {
		t.Errorf("Expected SSRC states to exist")
		return
	}

	if dc.dm[ssrc1].lastErr == dc.dm[ssrc2].lastErr {
		t.Errorf("Expected SSRC states to be independent")
	}
}
