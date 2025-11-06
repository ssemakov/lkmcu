package clock

import "testing"

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
