package media

type RingBuffer[T comparable] struct {
	data []T
	head int
	size int
	cap  int
}

// NewRingBuffer creates a new RingBuffer with the specified capacity.
// capacity must be greater than 0.
// T must be a comparable type.
func NewRingBuffer[T comparable](capacity int) *RingBuffer[T] {
	if capacity <= 0 {
		panic("capacity must be greater than 0")
	}

	return &RingBuffer[T]{
		data: make([]T, capacity),
		cap:  capacity,
	}
}

// Push adds a new element to the ring buffer.
// If the buffer is full, it overwrites the oldest element.
func (rb *RingBuffer[T]) Push(value T) {
	rb.data[rb.head] = value
	rb.head = (rb.head + 1) % rb.cap
	if rb.size < rb.cap {
		rb.size++
	}
}

// Values returns a slice containing all elements in the ring buffer in order from oldest to newest.
func (rb *RingBuffer[T]) Values() []T {
	values := make([]T, rb.size)
	if rb.size == 0 {
		return values
	}
	start := (rb.head - rb.size + rb.cap) % rb.cap
	for i := 0; i < rb.size; i++ {
		values[i] = rb.data[(start+i)%rb.cap]
	}

	return values
}

// Has checks if the ring buffer contains the specified key.
// Returns true if the key is found, false otherwise.
// key T must be comparable.
func (rb *RingBuffer[T]) Has(key T) bool {
	for _, k := range rb.Values() {
		if k == key {
			return true
		}
	}
	return false
}

// Clear removes all elements from the ring buffer.
func (rb *RingBuffer[T]) Clear() {
	rb.head = 0
	rb.size = 0
}
