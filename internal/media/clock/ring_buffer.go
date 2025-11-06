package clock

type RingBuffer[T any] struct {
	data []T
	head int
	size int
	cap  int
}

func NewRingBuffer[T any](capacity int) *RingBuffer[T] {
	return &RingBuffer[T]{
		data: make([]T, capacity),
		cap:  capacity,
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
