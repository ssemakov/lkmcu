package jitter

import "container/heap"

type PacketHeap struct {
	packets   []*timedPacket
	maxPacket *timedPacket
}

func (ph PacketHeap) Len() int { return len(ph.packets) }

func (ph PacketHeap) Less(i, j int) bool {
	return ph.packets[i].Less(ph.packets[j])
}

func (ph PacketHeap) Swap(i, j int) {
	ph.packets[i], ph.packets[j] = ph.packets[j], ph.packets[i]
}

func (ph *PacketHeap) Push(x interface{}) {
	ph.packets = append(ph.packets, x.(*timedPacket))
}

func (ph *PacketHeap) Pop() interface{} {
	old := ph.packets
	n := len(old)
	x := old[n-1]
	old[n-1] = nil // avoid memory leak
	ph.packets = old[0 : n-1]
	return x
}

func (ph *PacketHeap) PushPacket(tp *timedPacket) {
	heap.Push(ph, tp)

	if ph.maxPacket == nil || ph.maxPacket.Less(tp) {
		ph.maxPacket = tp
	}

}

func (ph *PacketHeap) PopPacket() *timedPacket {
	if ph.Len() == 0 {
		return nil
	}

	// The packets key is unique (PayloadTs, Seq), therefore the only time Max will be Pop'ed
	// is when it is the only packet in the heap and also min packet at the same time
	// in that case assign maxPacket to nil.
	if ph.PeekPacket() == ph.maxPacket {
		ph.maxPacket = nil
	}

	return heap.Pop(ph).(*timedPacket)
}

func (ph *PacketHeap) PeekPacket() *timedPacket {
	if ph == nil || ph.Len() == 0 {
		return nil
	}
	return ph.packets[0]
}

func (ph *PacketHeap) PeekMaxPacket() *timedPacket {
	return ph.maxPacket
}
