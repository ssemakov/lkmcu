package media

import "github.com/ssemakov/lkmcu/internal/layout"

type VideoCompositor interface {
	SetLayout(stage layout.Stage)
	Composite(dst []byte)
}
