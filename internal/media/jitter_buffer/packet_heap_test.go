package jitter_buffer

import (
	"container/heap"
	"testing"
	"time"
)

func newPacketHeap() *PacketHeap {
	ph := &PacketHeap{}
	heap.Init(ph)

	return ph
}

func TestPacketHeap(t *testing.T) {
	p1 := &Packet{Seq: 1, PayloadTs: time.Unix(0, 1000)}
	p2 := &Packet{Seq: 2, PayloadTs: time.Unix(0, 500)}
	p3 := &Packet{Seq: 3, PayloadTs: time.Unix(0, 1500)}

	ph := newPacketHeap()
	ph.PushPacket(p1)
	ph.PushPacket(p2)
	ph.PushPacket(p3)

	if ph.Len() != 3 {
		t.Fatalf("Len=%d want=3", ph.Len())
	}

	// PopPacket should return packets in order of PayloadTs
	popped := ph.PopPacket()
	if popped.Seq != 2 {
		t.Fatalf("first popped Seq=%d want=2", popped.Seq)
	}
	popped = ph.PopPacket()
	if popped.Seq != 1 {
		t.Fatalf("second popped Seq=%d want=1", popped.Seq)
	}
	popped = ph.PopPacket()
	if popped.Seq != 3 {
		t.Fatalf("third popped Seq=%d want=3", popped.Seq)
	}

	if ph.Len() != 0 {
		t.Fatalf("Len=%d want=0", ph.Len())
	}
}

func TestPacketHeap_PeekPacket(t *testing.T) {
	p1 := &Packet{Seq: 1, PayloadTs: time.Unix(0, 1000)}
	p2 := &Packet{Seq: 2, PayloadTs: time.Unix(0, 500)}

	ph := newPacketHeap()
	ph.PushPacket(p1)
	ph.PushPacket(p2)

	if ph.Len() != 2 {
		t.Fatalf("Len=%d want=2", ph.Len())
	}

	peeked := ph.PeekPacket()
	if peeked.Seq != 2 {
		t.Fatalf("peeked Seq=%d want=2", peeked.Seq)
	}

	if ph.Len() != 2 {
		t.Fatalf("Len=%d want=2 after PeekPacket", ph.Len())
	}
}

func TestPacketHeap_PeekMaxPacket(t *testing.T) {
	p1 := &Packet{Seq: 1, PayloadTs: time.Unix(0, 1000)}
	p2 := &Packet{Seq: 2, PayloadTs: time.Unix(0, 500)}
	p3 := &Packet{Seq: 3, PayloadTs: time.Unix(0, 2000)}

	ph := newPacketHeap()
	ph.PushPacket(p1)
	ph.PushPacket(p2)
	ph.PushPacket(p3)

	max := ph.PeekMaxPacket()
	if max == nil {
		t.Fatal("PeekMaxPacket returned nil, want packet")
	}
	if max.Seq != 3 {
		t.Fatalf("PeekMaxPacket Seq=%d want=3", max.Seq)
	}

	// Ensure heap is unchanged
	if ph.Len() != 3 {
		t.Fatalf("Len=%d want=3 after PeekMaxPacket", ph.Len())
	}
}

func TestPacketHeap_EqualTimestamps(t *testing.T) {
	p1 := &Packet{Seq: 1, PayloadTs: time.Unix(0, 1000)}
	p2 := &Packet{Seq: 2, PayloadTs: time.Unix(0, 1000)}
	p3 := &Packet{Seq: 3, PayloadTs: time.Unix(0, 1000)}

	ph := newPacketHeap()
	ph.PushPacket(p2)
	ph.PushPacket(p3)
	ph.PushPacket(p1)

	if ph.Len() != 3 {
		t.Fatalf("Len=%d want=3", ph.Len())
	}

	// PopPacket should return packets in order of Seq since PayloadTs are equal
	popped := ph.PopPacket()
	if popped.Seq != 1 {
		t.Fatalf("first popped Seq=%d want=1", popped.Seq)
	}
	popped = ph.PopPacket()
	if popped.Seq != 2 {
		t.Fatalf("second popped Seq=%d want=2", popped.Seq)
	}
	popped = ph.PopPacket()
	if popped.Seq != 3 {
		t.Fatalf("third popped Seq=%d want=3", popped.Seq)
	}

	if ph.Len() != 0 {
		t.Fatalf("Len=%d want=0", ph.Len())
	}
}
