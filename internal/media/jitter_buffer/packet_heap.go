package jitter_buffer

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

func (ph *PacketHeap) Peek() *Packet {
	if ph.Len() == 0 {
		return nil
	}
	return (*ph)[0]
}
