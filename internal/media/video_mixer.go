package media

import "livekit-mcu/internal/layout"

type VideoCompositor interface {
	SetLayout(stage layout.Stage)
	Composite(dst []byte)
}
