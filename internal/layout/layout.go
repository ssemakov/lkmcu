package layout

import (
  "encoding/json"
  "io"
  "net/http"
)

type Rect struct {
  X float32 `json:"x"`
  Y float32 `json:"y"`
  W float32 `json:"w"`
  H float32 `json:"h"`
  Z int     `json:"z"`
}

type Stage struct {
  Slots []Rect `json:"slots"`
}

func Default() Stage {
  return Stage{Slots: []Rect{
    {X: 0, Y: 0, W: 0.5, H: 0.5, Z: 0},
    {X: 0.5, Y: 0, W: 0.5, H: 0.5, Z: 0},
    {X: 0, Y: 0.5, W: 0.5, H: 0.5, Z: 0},
    {X: 0.5, Y: 0.5, W: 0.5, H: 0.5, Z: 0},
  }}
}

func HTTPHandler(stage *Stage, w http.ResponseWriter, r *http.Request) {
  switch r.Method {
  case http.MethodGet:
    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(stage)
  case http.MethodPost:
    body, _ := io.ReadAll(r.Body)
    var s Stage
    if err := json.Unmarshal(body, &s); err != nil || len(s.Slots) == 0 {
      w.WriteHeader(http.StatusBadRequest)
      _, _ = w.Write([]byte("invalid layout"))
      return
    }
    *stage = s
    w.WriteHeader(http.StatusNoContent)
  default:
    w.WriteHeader(http.StatusMethodNotAllowed)
  }
}
