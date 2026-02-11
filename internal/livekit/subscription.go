package livekit

type Subscription struct {
	TID    string
	PID    string
	Kind   string
	Source string

	// layer int // video quality layer
}

func (sub *Subscription) String() string {
	return "Subscription{ PID: " + sub.PID + ", TID: " + sub.TID + ", Kind: " + sub.Kind + ", Source: " + sub.Source + " }"
}
