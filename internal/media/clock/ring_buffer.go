package clock

type RingBuffer[T any] struct {
	data     []T
	head     int
	size     int
	cap      int
	equalsFn func(a, b T) bool
}

func NewRingBuffer[T any](capacity int, equalsFn func(a, b T) bool) *RingBuffer[T] {
	if capacity <= 0 {
		panic("capacity must be greater than 0")
	}
	if equalsFn == nil {
		panic("equalsFn must not be nil")
	}

	return &RingBuffer[T]{
		data:     make([]T, capacity),
		cap:      capacity,
		equalsFn: equalsFn,
	}
}

func (rb *RingBuffer[T]) Push(value T) {
	rb.data[rb.head] = value
	rb.head = (rb.head + 1) % rb.cap
	if rb.size < rb.cap {
		rb.size++
	}
}

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

func (rb *RingBuffer[T]) Has(key T) bool {
	for _, k := range rb.data {
		if rb.equalsFn(k, key) {
			return true
		}
	}
	return false
}
