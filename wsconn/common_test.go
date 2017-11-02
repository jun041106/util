// Copyright 2017 Apcera Inc. All rights reserved.

package wsconn

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/apcera/util/testtool"
	"github.com/gorilla/websocket"
)

func getTestWebsockets(t *testing.T) (*websocket.Conn, *websocket.Conn) {
	serverWSChan := make(chan *websocket.Conn)
	ts := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			upgrader := websocket.Upgrader{
				ReadBufferSize:  1024,
				WriteBufferSize: 1024,
			}
			upgrader.CheckOrigin = func(r *http.Request) bool { return true }
			serverWS, err := upgrader.Upgrade(w, r, nil)
			testtool.TestExpectSuccess(t, err)
			serverWSChan <- serverWS
		},
	))
	// Normally the server would block until response, but the hijacked
	// connections are forgotten.
	// https://golang.org/src/net/http/httptest/server.go?s=6920:6961#L325
	defer ts.Close()
	defer close(serverWSChan)

	wsurl, err := url.ParseRequestURI(ts.URL)
	testtool.TestExpectSuccess(t, err)

	wsurl.Scheme = "ws"

	clientWS, _, err := websocket.DefaultDialer.Dial(wsurl.String(), nil)
	testtool.TestExpectSuccess(t, err)
	var serverWS *websocket.Conn

	select {
	case <-time.After(10 * time.Second):
		t.Fatal("Timed out waiting for server WS")
	case serverWS = <-serverWSChan:
	}
	return serverWS, clientWS
}
