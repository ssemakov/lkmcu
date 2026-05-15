package jitter

import (
	"container/heap"
	"testing"
	"time"

	"github.com/ssemakov/lkmcu/internal/media/packet"
)

func newPacketHeap() *PacketHeap {
	ph := &PacketHeap{}
	heap.Init(ph)

	return ph
}

func TestPacketHeap(t *testing.T) {
	p1 := &timedPacket{Packet: packet.Packet{Seq: 1}, PayloadTS: time.Unix(0, 1000)}
	p2 := &timedPacket{Packet: packet.Packet{Seq: 2}, PayloadTS: time.Unix(0, 500)}
	p3 := &timedPacket{Packet: packet.Packet{Seq: 3}, PayloadTS: time.Unix(0, 1500)}

	ph := newPacketHeap()
	ph.PushPacket(p1)
	ph.PushPacket(p2)
	ph.PushPacket(p3)

	if ph.Len() != 3 {
		t.Fatalf("Len=%d want=3", ph.Len())
	}

	// PopPacket should return packets in order of PayloadTS
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
	p1 := &timedPacket{Packet: packet.Packet{Seq: 1}, PayloadTS: time.Unix(0, 1000)}
	p2 := &timedPacket{Packet: packet.Packet{Seq: 2}, PayloadTS: time.Unix(0, 500)}

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
	p1 := &timedPacket{Packet: packet.Packet{Seq: 1}, PayloadTS: time.Unix(0, 1000)}
	p2 := &timedPacket{Packet: packet.Packet{Seq: 2}, PayloadTS: time.Unix(0, 500)}
	p3 := &timedPacket{Packet: packet.Packet{Seq: 3}, PayloadTS: time.Unix(0, 2000)}

	ph := newPacketHeap()
	ph.PushPacket(p1)
	ph.PushPacket(p2)
	ph.PushPacket(p3)

	len := ph.Len()
	for i := 0; i < len; i++ {
		max := ph.PeekMaxPacket()
		if max == nil {
			t.Fatal("PeekMaxPacket returned nil, want packet")
		}
		if max.Seq != 3 {
			t.Fatalf("PeekMaxPacket Seq=%d want=3", max.Seq)
		}

		// Ensure heap is unchanged
		if ph.Len() != len-i {
			t.Fatalf("Len=%d want=3 after PeekMaxPacket", ph.Len())
		}

		ph.PopPacket()
	}
}

func TestPacketHeap_EqualTimestamps(t *testing.T) {
	p1 := &timedPacket{Packet: packet.Packet{Seq: 1}, PayloadTS: time.Unix(0, 1000)}
	p2 := &timedPacket{Packet: packet.Packet{Seq: 2}, PayloadTS: time.Unix(0, 1000)}
	p3 := &timedPacket{Packet: packet.Packet{Seq: 3}, PayloadTS: time.Unix(0, 1000)}

	ph := newPacketHeap()
	ph.PushPacket(p2)
	ph.PushPacket(p3)
	ph.PushPacket(p1)

	if ph.Len() != 3 {
		t.Fatalf("Len=%d want=3", ph.Len())
	}

	// PopPacket should return packets in order of Seq since PayloadTS are equal
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
