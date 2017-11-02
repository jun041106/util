// Copyright 2017 Apcera Inc. All rights reserved.

package wsconn

import (
	"io/ioutil"
	"net"
	"testing"
	"time"

	"github.com/apcera/util/testtool"
	"github.com/apcera/util/timeoutwg"
	"github.com/gorilla/websocket"
)

func TestProxy(t *testing.T) {
	sample1 := "Hello World"

	aWS, bWS := getTestWebsockets(t)
	cWS, dWS := getTestWebsockets(t)
	var twg timeoutwg.TimeoutWaitGroup
	twg.Add(1)
	go func() {
		defer twg.Done()
		w, err := aWS.NextWriter(websocket.BinaryMessage)
		testtool.TestExpectSuccess(t, err)
		_, err = w.Write([]byte(sample1))
		testtool.TestExpectSuccess(t, err)
		err = w.Close()
		testtool.TestExpectSuccess(t, err)

		aWS.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, "Closing"),
			time.Now().Add(5*time.Second))
	}()
	twg.Add(1)
	go func() {
		defer twg.Done()
		msgType, r, err := dWS.NextReader()
		testtool.TestExpectSuccess(t, err)
		out, err := ioutil.ReadAll(r)
		testtool.TestExpectSuccess(t, err)
		testtool.TestEqual(t, string(out), sample1)
		testtool.TestEqual(t, msgType, websocket.BinaryMessage)
		dWS.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, "Closing"),
			time.Now().Add(5*time.Second))
	}()

	err := Proxy(bWS, cWS)
	testtool.TestExpectSuccess(t, err)
	testtool.TestEqual(t, twg.Wait(5*time.Second), 0)
	for _, ws := range []*websocket.Conn{aWS, bWS, cWS, dWS} {
		err = ws.Close()
		testtool.TestExpectSuccess(t, err)
	}
}

func TestProxyTimeout(t *testing.T) {
	aWS, bWS := getTestWebsockets(t)
	cWS, dWS := getTestWebsockets(t)

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
	defer cWS.Close()
	defer dWS.Close()
	// dWS will stay inactive, not responding to pings from the proxy.
	dWS.SetPingHandler(func(_ string) error { return nil })

	var twg timeoutwg.TimeoutWaitGroup

	twg.Add(1)
	go func() {
		defer twg.Done()
		err := Proxy(bWS, cWS)
		testtool.TestExpectError(t, err)
		netErr, ok := err.(net.Error)
		testtool.TestTrue(t, ok)
		testtool.TestTrue(t, netErr.Timeout())
	}()

	testtool.TestEqual(t, twg.Wait(3*time.Second), 0)
}
