package media

import (
	"testing"
)

func TestRingBuffer_Values_Empty(t *testing.T) {
	rb := NewRingBuffer[int](5)

	values := rb.Values()
	if rb.size != 0 {
		t.Errorf("Expected size 0, got %d", rb.size)
	}

	if rb.cap != 5 {
		t.Errorf("Expected capacity 5, got %d", rb.cap)
	}

	if len(values) != 0 {
		t.Errorf("Expected empty slice, got %v", values)
	}
}

func TestRingBuffer_Values_Partial(t *testing.T) {
	rb := NewRingBuffer[int](5)
	rb.Push(1)
	rb.Push(2)
	rb.Push(3)

	values := rb.Values()

	if rb.size != 3 {
		t.Errorf("Expected size 3, got %d", rb.size)
	}

	expected := []int{1, 2, 3}
	if len(values) != len(expected) {
		t.Fatalf("Expected length %d, got %d", len(expected), len(values))
	}

	for i, v := range expected {
		if values[i] != v {
			t.Errorf("At index %d, expected %d, got %d", i, v, values[i])
		}
	}
}

func TestRingBuffer_Values_FullAndWrap(t *testing.T) {
	rb := NewRingBuffer[int](3)
	rb.Push(1)
	rb.Push(2)
	rb.Push(3)
	rb.Push(4) // This should overwrite the oldest value (1)

	values := rb.Values()

	if rb.size != 3 {
		t.Errorf("Expected size 3, got %d", rb.size)
	}

	expected := []int{2, 3, 4}
	if len(values) != len(expected) {
		t.Fatalf("Expected length %d, got %d", len(expected), len(values))
	}

	for i, v := range expected {
		if values[i] != v {
			t.Errorf("At index %d, expected %d, got %d", i, v, values[i])
		}
	}
}

func TestRingBuffer_Has(t *testing.T) {
	rb := NewRingBuffer[int](3)
	rb.Push(10)
	rb.Push(20)
	rb.Push(30)

	tests := []struct {
		val      int
		expected bool
	}{
		{10, true},
		{20, true},
		{30, true},
		{40, false},
	}

	for _, tt := range tests {
		if got := rb.Has(tt.val); got != tt.expected {
			t.Errorf("Has(%d): expected %v, got %v", tt.val, tt.expected, got)
		}
	}

	// Test after wrap
	rb.Push(40) // Now buffer is [20, 30, 40]
	if rb.Has(10) {
		t.Errorf("Has(10): expected false after wrap, got true")
	}
	if !rb.Has(40) {
		t.Errorf("Has(40): expected true after wrap, got false")
	}
}
