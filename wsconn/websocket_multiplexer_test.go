// Copyright 2017 Apcera Inc. All rights reserved.

package wsconn

import (
	"io"
	"io/ioutil"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/apcera/util/testtool"
	"github.com/apcera/util/timeoutwg"
	"github.com/gorilla/websocket"
)

func TestMultiplexerBasicFunctionality(t *testing.T) {
	sample1 := "Hello World"
	sample2 := "Hola Mundo"

	const sample3Size = 1024
	const sample3Count = 1024
	sample3 := strings.Repeat("!", sample3Size)

	serverWS, clientWS := getTestWebsockets(t)

	sMux := NewMultiplexer(serverWS)
	defer sMux.Close()

	// This test verifies we can send and receive and that there's
	// no cross-over between channels
	serverR0, serverR0WPipe := io.Pipe()
	serverR0err, err := sMux.HandleInboundStream(0, serverR0WPipe)
	testtool.TestExpectSuccess(t, err)
	serverR1, serverR1WPipe := io.Pipe()
	serverR1err, err := sMux.HandleInboundStream(1, serverR1WPipe)
	testtool.TestExpectSuccess(t, err)
	serverW3 := sMux.GetStreamWriter(3)
	sMux.Start()

	cMux := NewMultiplexer(clientWS)
	defer cMux.Close()
	clientR3, clientR3WPipe := io.Pipe()
	clientR3err, err := cMux.HandleInboundStream(3, clientR3WPipe)
	testtool.TestExpectSuccess(t, err)
	clientW0 := cMux.GetStreamWriter(0)
	// try using AddOutboundStream w/ a pipe. Semantics should be essentially
	// the same as a writer.
	clientW1R, clientW1 := io.Pipe()
	clientW1err := cMux.AddOutboundStream(1, clientW1R)
	cMux.Start()

	var twg timeoutwg.TimeoutWaitGroup
	twg.Add(1)
	go func() {
		defer twg.Done()
		buf := make([]byte, len(sample1))
		n, err := io.ReadFull(serverR0, buf)
		testtool.TestExpectSuccess(t, err)
		testtool.TestEqual(t, n, len(buf))
		testtool.TestEqual(t, n, len(sample1))

		buf = make([]byte, len(sample1))
		n, err = io.ReadFull(serverR0, buf)
		testtool.TestExpectSuccess(t, err)
		testtool.TestEqual(t, n, len(buf))
		testtool.TestEqual(t, n, len(sample1))

		// We expect this to end in an EOF
		_, err = serverR0.Read(buf)
		testtool.TestEqual(t, err, io.EOF)
	}()

	twg.Add(1)
	go func() {
		defer twg.Done()

		for i := 0; i < sample3Count; i++ {
			n, err := serverW3.Write([]byte(sample3))
			testtool.TestExpectSuccess(t, err)
			testtool.TestEqual(t, n, len(sample3))
		}

		err = serverW3.Close()
		testtool.TestExpectSuccess(t, err)

		// Wait for the managed writer to finish and then close the channel.
		<-clientW1err
		// Let the remote side know we're done.
		err = sMux.SendClose(websocket.CloseNormalClosure, "Done!")
		testtool.TestExpectSuccess(t, err)
	}()

	twg.Add(1)
	go func() {
		defer twg.Done()
		buf := make([]byte, len(sample2))
		n, err := io.ReadFull(serverR1, buf)
		testtool.TestEqual(t, n, len(buf))
		testtool.TestEqual(t, n, len(sample2))
		testtool.TestExpectSuccess(t, err)

		// We expect this to end in an EOF
		_, err = serverR1.Read(buf)
		testtool.TestEqual(t, err, io.EOF)
	}()

	twg.Add(1)
	go func() {
		defer twg.Done()
		for i := 0; i < sample3Count; i++ {
			buf := make([]byte, len(sample3))
			n, err := io.ReadFull(clientR3, buf)
			testtool.TestEqual(t, n, len(buf))
			testtool.TestEqual(t, n, len(sample3))
			testtool.TestExpectSuccess(t, err)
		}
	}()

	twg.Add(1)
	go func() {
		defer twg.Done()
		n, err := clientW0.Write([]byte(sample1))
		testtool.TestExpectSuccess(t, err)
		testtool.TestEqual(t, n, len(sample1))

		n, err = clientW1.Write([]byte(sample2))
		testtool.TestExpectSuccess(t, err)
		testtool.TestEqual(t, n, len(sample2))
		err = clientW1.Close()
		testtool.TestExpectSuccess(t, err)

		n, err = clientW0.Write([]byte(sample1))
		testtool.TestExpectSuccess(t, err)
		testtool.TestEqual(t, n, len(sample1))

		err = clientW0.Close()
		testtool.TestExpectSuccess(t, err)

		// Let the remote side know we're done.
		err = cMux.SendClose(websocket.CloseNormalClosure, "Done!")
		testtool.TestExpectSuccess(t, err)
	}()

	// Watch the local error channels.
	twg.Add(4)
	go func() {
		defer twg.Done()
		testtool.TestExpectSuccess(t, <-serverR0err)
	}()
	go func() {
		defer twg.Done()
		testtool.TestExpectSuccess(t, <-serverR1err)
	}()
	go func() {
		defer twg.Done()
		testtool.TestExpectSuccess(t, <-clientR3err)
	}()
	go func() {
		defer twg.Done()
		testtool.TestExpectSuccess(t, <-clientW1err)
	}()

	twg.Add(1)
	go func() {
		defer twg.Done()
		sMux.WaitForRemote()
		cMux.WaitForRemote()
	}()

	if twg.Wait(10*time.Second) != 0 {
		t.Fatalf("Test timed out.")
	}

}

func TestMultiplexerConnectionTimeout(t *testing.T) {
	aWS, bWS := getTestWebsockets(t)

	oldDefaultPingInterval := defaultPingInterval
	defaultPingInterval = 200 * time.Millisecond
	oldDefaultReadTimeout := defaultReadTimeout
	defaultReadTimeout = 1 * time.Second
	oldDefaultWriteTimeout := defaultWriteTimeout
	defaultWriteTimeout = 1 * time.Second
	defer func() {
		defaultPingInterval = oldDefaultPingInterval
		defaultReadTimeout = oldDefaultReadTimeout
		defaultWriteTimeout = oldDefaultWriteTimeout
	}()

	defer aWS.Close()
	defer bWS.Close()
	// bWS will stay inactive, not responding to pings from the proxy.
	bWS.SetPingHandler(func(_ string) error { return nil })
	// WebsocketConnection runs a pinger goroutine
	aMux := NewMultiplexer(aWS)
	readErrChan, err := aMux.HandleInboundStream(0, NopWriteCloser(ioutil.Discard))
	testtool.TestExpectSuccess(t, err)

	var twg timeoutwg.TimeoutWaitGroup

	twg.Add(1)
	go func() {
		defer twg.Done()
		err := <-readErrChan
		testtool.TestExpectSuccess(t, err)
		err = aMux.WaitForRemote()
		testtool.TestExpectError(t, err)
		netErr, ok := err.(net.Error)
		testtool.TestTrue(t, ok)
		testtool.TestTrue(t, netErr.Timeout())
	}()

	aMux.Start()

	remain := twg.Wait(3 * time.Second)
	if remain != 0 {
		t.Fatalf("Test timed out with %d goroutines still active.", remain)
	}
}

func TestMultiplexerChannelCloses(t *testing.T) {
	sample1 := "Hello World"
	sample2 := "Hola Mundo"

	serverWS, clientWS := getTestWebsockets(t)

	sMux := NewMultiplexer(serverWS)

	// This test verifies we can send and receive and that there's
	// no cross-over between channels
	serverW0 := sMux.GetStreamWriter(0)
	sMux.Start()

	cMux := NewMultiplexer(clientWS)
	defer cMux.Close()
	clientR0Err, err := cMux.HandleInboundStream(0, NopWriteCloser(ioutil.Discard))
	testtool.TestExpectSuccess(t, err)
	cMux.Start()

	var twg timeoutwg.TimeoutWaitGroup
	twg.Add(1)
	go func() {
		defer twg.Done()
		_, err := serverW0.Write([]byte(sample1))
		testtool.TestExpectSuccess(t, err)
		err = serverW0.Close()
		testtool.TestExpectSuccess(t, err)

		gotClose := make(chan struct{})
		go func() {
			// this should end in EOF when it hits the writer's close
			err := <-clientR0Err
			testtool.TestExpectSuccess(t, err)
			close(gotClose)
		}()
		select {
		case <-gotClose:
		case <-time.After(500 * time.Millisecond):
			t.Fatal("Reader should have closed after write peer did.")
		}

		// Writing to a closed Writer is an error
		_, err = serverW0.Write([]byte(sample2))
		testtool.TestExpectError(t, err)
	}()

	if twg.Wait(10*time.Second) > 0 {
		t.Fatalf("Websocket Closed, but peer kept blocking.")
	}
}
