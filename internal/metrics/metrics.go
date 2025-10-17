package metrics

import "net/http"

func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })
}

type Metrics struct {
	// placeholder for metrics fields
}

func NewMetrics() *Metrics {
	return &Metrics{}
}
