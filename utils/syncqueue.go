package utils

type Queue struct {
	capacity int
	q        []int
}

// FifoQueue
type FifoQueue interface {
	Push(int) bool
	BulkPush([]int) bool
	Pop() (int, bool)
	Length() int
}

func (q *Queue) Length() int {
	return len(q.q)
}

func (q *Queue) BulkPush(items []int) (ok bool) {
	if (len(q.q) + len(items)) < q.capacity {
		q.q = append(q.q, items...)

		return true
	}

	return false
}

// Push inserts the item into the queue
func (q *Queue) Push(item int) (ok bool) {
	if len(q.q) < q.capacity {
		q.q = append(q.q, item)

		return true
	}

	return false
}

// Pop removes the oldest element from the queue
func (q *Queue) Pop() (item int, ok bool) {
	if len(q.q) > 0 {
		item := q.q[0]
		q.q = q.q[1:]

		return item, true
	}

	return 0, false
}

// CreateQueue creates an empty queue with desired capacity
func CreateQueue(capacity int) *Queue {
	return &Queue{
		capacity: capacity,
		q:        make([]int, 0, capacity),
	}
}
