package pool

import (
	"sync/atomic"
	"unsafe"
)

type Queue struct {
	head unsafe.Pointer
	tail unsafe.Pointer
	len  uint64
}
type node struct {
	value interface{}
	next  unsafe.Pointer
}

func NewQueue() *Queue {
	n := unsafe.Pointer(&node{})
	return &Queue{head: n, tail: n, len: 0}
}

func (q *Queue) Enqueue(v interface{}) {
	defer func() {
		atomic.AddUint64(&(q.len), 1)
	}()
	n := &node{value: v}
	for {
		tail := load(&q.tail)
		next := load(&tail.next)
		if tail == load(&q.tail) {
			if next == nil {
				if cas(&tail.next, next, n) {
					cas(&q.tail, tail, n)
					return
				}
			} else {
				cas(&q.tail, tail, next)
			}
		}
	}
}

func (q *Queue) Dequeue() interface{} {
	var data interface{} = nil
	defer func() {
		if data != nil {
			atomic.AddUint64(&(q.len), ^uint64(0))
		}
	}()
	for {
		head := load(&q.head)
		tail := load(&q.tail)
		next := load(&head.next)
		if head == load(&q.head) {
			if head == tail {
				if next == nil {
					return nil
				}
				cas(&q.tail, tail, next)
			} else {
				v := next.value
				if cas(&q.head, head, next) {
					data = v
					return v
				}
			}
		}
	}
}
func (q *Queue) Len() uint64 {
	return atomic.LoadUint64(&(q.len))
}
func load(p *unsafe.Pointer) *node {
	return (*node)(atomic.LoadPointer(p))
}
func cas(p *unsafe.Pointer, old, new *node) bool {
	return atomic.CompareAndSwapPointer(
		p, unsafe.Pointer(old), unsafe.Pointer(new))
}
