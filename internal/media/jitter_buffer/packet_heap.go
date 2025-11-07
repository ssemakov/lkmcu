package jitter_buffer

import (
	"time"
)

type Packet struct {
	PayloadTs time.Time
	Seq       uint16
	// Payload   []byte --- IGNORE ---
}

type PacketHeap []*Packet

func (ph PacketHeap) Len() int { return len(ph) }

func (ph PacketHeap) Less(i, j int) bool {
	// min heap based on PayloadTs
	if ph[i].PayloadTs.Equal(ph[j].PayloadTs) {
		return ph[i].Seq < ph[j].Seq
	}
	return ph[i].PayloadTs.Before(ph[j].PayloadTs)
}

func (ph PacketHeap) Swap(i, j int) {
	ph[i], ph[j] = ph[j], ph[i]
}

func (ph *PacketHeap) Push(x interface{}) {
	*ph = append(*ph, x.(*Packet))
}

func (ph *PacketHeap) Pop() interface{} {
	old := *ph
	n := len(old)
	x := old[n-1]
	*ph = old[0 : n-1]
	return x
}
