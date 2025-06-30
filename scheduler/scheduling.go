package scheduler

import (
	"container/heap"
	"fmt"
	"time"
)

type Event[T any] struct {
	event     T
	scheduler *Scheduler[T]
	at        int64 // UNIX time in milliseconds
	index     int   // index of the element in the heap
}

type Scheduler[T any] struct {
	q      []*Event[T]
	ready  chan T
	stop   chan struct{}
	wakeup chan struct{}
}

// sort.Interface
func (sched Scheduler[T]) Len() int { return len(sched.q) }

func (sched Scheduler[T]) Less(i, j int) bool {
	return sched.q[i].at < sched.q[j].at
}

func (sched Scheduler[T]) Swap(i, j int) {
	sched.q[i], sched.q[j] = sched.q[j], sched.q[i]
	sched.q[i].index = i
	sched.q[j].index = j
}

// heap.Interface
func (sched *Scheduler[T]) Push(x any) {
	event := x.(*Event[T])
	event.index = len(sched.q)
	sched.q = append(sched.q, event)
}

func (sched *Scheduler[T]) Pop() any {
	n := len(sched.q)
	event := sched.q[n-1]
	sched.q[n-1] = nil // don't stop the GC from reclaiming the item eventually
	sched.q = sched.q[:n-1]
	event.index = -1 // for safety
	event.scheduler = nil
	return event
}

func NewScheduler[T any](ready chan T) *Scheduler[T] {
	sched := &Scheduler[T]{
		q:     make([]*Event[T], 0),
		ready: ready,
	}
	heap.Init(sched) // Not that useful
	return sched
}

func (sched *Scheduler[T]) Start() (chan struct{}, error) {

	// XXX need locking
	if sched.stop != nil {
		return nil, fmt.Errorf("already started")
	}

	stopped := make(chan struct{})
	sched.stop = make(chan struct{})
	sched.wakeup = make(chan struct{})

	go func() {
		for {
			empty, delay := sched.doDequeue()
			if empty {
				select {
				case <-sched.wakeup:
					break
				case <-sched.stop:
					goto done
				}
			} else {
				select {
				case <-time.After(delay):
					break
				case <-sched.wakeup:
					break
				case <-sched.stop:
					goto done

				}
			}
		}
	done:
		close(sched.wakeup)
		sched.wakeup = nil
		close(stopped)
	}()

	return stopped, nil
}

func (sched *Scheduler[T]) Stop() {
	if sched.stop != nil {
		close(sched.stop)
		sched.stop = nil
	}
}

func (sched *Scheduler[T]) doWakeup() {
	if sched.wakeup != nil {
		sched.wakeup <- struct{}{}
	}
}

func (sched *Scheduler[T]) doDequeue() (bool, time.Duration) {
	now := time.Now().UnixMilli()
	for len(sched.q) != 0 {
		event := sched.q[0]
		if event.at > now {
			// Try with a more recent time
			now = time.Now().UnixMilli()
			delay := event.at - now
			if delay > 0 {
				return false, time.Duration(delay) * time.Millisecond
			}
		}
		sched.ready <- heap.Pop(sched).(*Event[T]).event
	}
	return true, -1
}

func (event *Event[T]) Reschedule(at time.Time) {
	event.at = at.UnixMilli()
	heap.Fix(event.scheduler, event.index)
	event.scheduler.doWakeup()
}

func (event *Event[T]) Cancel() {
	heap.Remove(event.scheduler, event.index)
	event.at = -1
	event.index = -1
	event.scheduler.doWakeup()
}

func (sched *Scheduler[T]) ScheduleAt(event T, at time.Time) *Event[T] {
	evt := &Event[T]{
		scheduler: sched,
		event:     event,
		at:        at.UnixMilli(),
	}
	heap.Push(sched, evt)
	sched.doWakeup()
	return evt
}

func (sched *Scheduler[T]) ScheduleAfter(event T, delay time.Duration) *Event[T] {
	return sched.ScheduleAt(event, time.Now().Add(delay))
}
