package vm

import (
	"sync"
	"time"
)

type Channel struct {
	buffer   []Value
	capacity int
	mutex    sync.Mutex
	sendCond *sync.Cond
	recvCond *sync.Cond
	closed   bool
}

func NewChannel(capacity int) *Channel {
	ch := &Channel{
		buffer:   make([]Value, 0, capacity),
		capacity: capacity,
	}
	ch.sendCond = sync.NewCond(&ch.mutex)
	ch.recvCond = sync.NewCond(&ch.mutex)
	return ch
}

func (ch *Channel) Send(value Value) bool {
	ch.mutex.Lock()
	defer ch.mutex.Unlock()

	if ch.closed {
		return false
	}

	for len(ch.buffer) >= ch.capacity {
		ch.sendCond.Wait()
		if ch.closed {
			return false
		}
	}

	ch.buffer = append(ch.buffer, value)
	ch.recvCond.Signal()
	return true
}

func (ch *Channel) Receive() (Value, bool) {
	ch.mutex.Lock()
	defer ch.mutex.Unlock()

	for len(ch.buffer) == 0 {
		if ch.closed {
			return nil, false
		}
		ch.recvCond.Wait()
	}

	value := ch.buffer[0]
	ch.buffer = ch.buffer[1:]
	ch.sendCond.Signal()
	return value, true
}

func (ch *Channel) Close() {
	ch.mutex.Lock()
	defer ch.mutex.Unlock()
	ch.closed = true
	ch.sendCond.Broadcast()
	ch.recvCond.Broadcast()
}

type SelectCase struct {
	Channel *Channel
	Value   Value
	IsSend  bool
}

func Select(cases []SelectCase) (int, bool) {
	for i, c := range cases {
		if c.IsSend {
			if c.Channel.Send(c.Value) {
				return i, true
			}
		} else {
			if value, ok := c.Channel.Receive(); ok {
				cases[i].Value = value
				return i, true
			}
		}
	}
	return -1, false
}

type Timer struct {
	duration time.Duration
	done     chan struct{}
}

func NewTimer(duration time.Duration) *Timer {
	t := &Timer{
		duration: duration,
		done:     make(chan struct{}),
	}
	go func() {
		time.Sleep(duration)
		close(t.done)
	}()
	return t
}

func (t *Timer) Done() <-chan struct{} {
	return t.done
}
