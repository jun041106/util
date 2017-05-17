// Copyright 2017 Apcera Inc. All rights reserved.

package testtool

import (
	"testing"
	"time"
)

// AssertChanReceive ensures that the given channel received the
// specified number of messages. The supplied timeout specifies the
// timeout on each expected read. It is not a cumulative timeout for
// all reads. AssertChanReceive requires a name of the function that
// should write to the channel in order to provide a useful error
// message.
func AssertChanReceive(
	t *testing.T,
	ch chan struct{},
	name string,
	timeout time.Duration,
	numSends int,
) {
	for i := 0; i < numSends; i++ {
		select {
		case <-ch:
		case <-time.After(timeout):
			t.Fatalf("%s was never called", name)
		}
	}
}

// AssertChanNoreceive ensures that the specified channel never
// received a message. AssertChanNoReceive requires a name of the
// function that should not write to the channel in order to provide a
// useful error message.
func AssertChanNoReceive(
	t *testing.T,
	ch chan struct{},
	name string,
	timeout time.Duration,
) {
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatalf("%s should not have been called", name)
		}
	case <-time.After(timeout):
	}
}

// SendWithCancel expects a channel to write a signal to, and also a
// channel that indicates when the signal listener is no longer
// listening.
func SendWithCancel(w chan<- struct{}, done <-chan struct{}) {
	select {
	case w <- struct{}{}:
		return
	case <-done:
		return
	}
}
