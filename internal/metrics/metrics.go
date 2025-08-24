package metrics

import "net/http"

func Handler() http.Handler { return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) }) }
