// Copyright 2017 Apcera Inc. All rights reserved.

package timeoutwg

import (
	"sync"
	"time"
)

// TimeoutWaitGroup works just like a normal wait-group, but Wait will exit and
// return true if all tasks have not declared Done() before the timeout expires.
// It's most useful in tests, but can also be useful for waiters which want to
// periodically report progress (number of remaining workers).
type TimeoutWaitGroup struct {
	workers int
	mutex   sync.Mutex
	last    chan struct{}
}

// Add increments the wait group's count of active workers by delta.
func (twg *TimeoutWaitGroup) Add(delta int) {
	twg.mutex.Lock()
	defer twg.mutex.Unlock()
	if twg.workers == 0 {
		// waitgroups are reusable once workers is 0
		twg.last = nil
	} else if twg.workers > 0 && twg.last != nil {
		// This mimics normal WG behavior. Cannot add after Wait() has been
		// called, unless workers is 0.  Prevents races on Wait()
		panic("TimeoutWaitGroup misuse: Add called concurrently with Wait")
	}
	twg.workers += delta
}

// Done decrements the wait group count by one.
func (twg *TimeoutWaitGroup) Done() {
	twg.mutex.Lock()
	defer twg.mutex.Unlock()
	twg.workers--
	if twg.workers < 0 {
		panic("TimeoutWaitGroup: negative counter")
	}
	if twg.workers == 0 && twg.last != nil {
		close(twg.last)
	}
}

// Wait blocks until waitDuration has passed or all workers have called Done(),
// which ever comes first.  Wait returns the number of
func (twg *TimeoutWaitGroup) Wait(waitDuration time.Duration) int {
	twg.mutex.Lock()
	if twg.workers == 0 {
		twg.mutex.Unlock()
		return 0
	}
	// Don't make the chan unless its needed. This lets us avoid a constructor
	if twg.last == nil {
		twg.last = make(chan struct{})
	}
	timeout := time.After(waitDuration)
	twg.mutex.Unlock()
	select {
	case <-twg.last:
	case <-timeout:
	}
	twg.mutex.Lock()
	defer twg.mutex.Unlock()
	return twg.workers
}
