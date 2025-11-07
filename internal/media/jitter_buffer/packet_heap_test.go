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
	heap.Push(ph, p1)
	heap.Push(ph, p2)
	heap.Push(ph, p3)

	if ph.Len() != 3 {
		t.Fatalf("Len=%d want=3", ph.Len())
	}

	// Pop should return packets in order of PayloadTs
	popped := heap.Pop(ph).(*Packet)
	if popped.Seq != 2 {
		t.Fatalf("first popped Seq=%d want=2", popped.Seq)
	}
	popped = heap.Pop(ph).(*Packet)
	if popped.Seq != 1 {
		t.Fatalf("second popped Seq=%d want=1", popped.Seq)
	}
	popped = heap.Pop(ph).(*Packet)
	if popped.Seq != 3 {
		t.Fatalf("third popped Seq=%d want=3", popped.Seq)
	}

	if ph.Len() != 0 {
		t.Fatalf("Len=%d want=0", ph.Len())
	}
}

func TestPacketHeap_EqualTimestamps(t *testing.T) {
	p1 := &Packet{Seq: 1, PayloadTs: time.Unix(0, 1000)}
	p2 := &Packet{Seq: 2, PayloadTs: time.Unix(0, 1000)}
	p3 := &Packet{Seq: 3, PayloadTs: time.Unix(0, 1000)}

	ph := newPacketHeap()
	heap.Push(ph, p2)
	heap.Push(ph, p3)
	heap.Push(ph, p1)

	if ph.Len() != 3 {
		t.Fatalf("Len=%d want=3", ph.Len())
	}

	// Pop should return packets in order of Seq since PayloadTs are equal
	popped := heap.Pop(ph).(*Packet)
	if popped.Seq != 1 {
		t.Fatalf("first popped Seq=%d want=1", popped.Seq)
	}
	popped = heap.Pop(ph).(*Packet)
	if popped.Seq != 2 {
		t.Fatalf("second popped Seq=%d want=2", popped.Seq)
	}
	popped = heap.Pop(ph).(*Packet)
	if popped.Seq != 3 {
		t.Fatalf("third popped Seq=%d want=3", popped.Seq)
	}

	if ph.Len() != 0 {
		t.Fatalf("Len=%d want=0", ph.Len())
	}
}
