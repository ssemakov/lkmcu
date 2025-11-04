package clock

import (
	"testing"

	"github.com/ssemakov/lkmcu/internal/logging"
	"github.com/ssemakov/lkmcu/internal/metrics"
)

func TestDrift_NewDriftController(t *testing.T) {
	dc := NewDriftController(metrics.NewMetrics(), logging.NewNullLogger("test", "debug"))

	if dc == nil {
		t.Errorf("Expected non-nil DriftController")
	}

	defaultCfg := DefaultDriftConfig()
	if dc.cfg != defaultCfg {
		t.Errorf("Expected default config to be set")
	}
}
