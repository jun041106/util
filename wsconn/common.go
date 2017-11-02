// Copyright 2017 Apcera Inc. All rights reserved.

package wsconn

import (
	"io"
	"net"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var (
	defaultReadTimeout  = 60 * time.Second
	defaultWriteTimeout = 10 * time.Second
	defaultPingInterval = 10 * time.Second
)

// Conn is an interface which a websocket library should implement to be
// compatible with this wrapper.
type Conn interface {
	WriteControl(messageType int, data []byte, deadline time.Time) error
	NextReader() (messageType int, r io.Reader, err error)
	NextWriter(messageType int) (io.WriteCloser, error)

	LocalAddr() net.Addr
	RemoteAddr() net.Addr

	SetReadDeadline(t time.Time) error
	SetWriteDeadline(t time.Time) error
	SetPingHandler(h func(string) error)
	SetPongHandler(h func(string) error)
	SetCloseHandler(h func(code int, text string) error)
	Close() error
}

func makePingHandler(src Conn, mutex *sync.Mutex) func(string) error {
	return func(_ string) error {
		mutex.Lock()
		defer mutex.Unlock()
		return src.WriteControl(
			websocket.PongMessage,
			[]byte{},
			time.Now().Add(defaultWriteTimeout))
	}
}

func makePongHandler(src Conn) func(string) error {
	return func(_ string) error {
		return src.SetReadDeadline(time.Now().Add(defaultReadTimeout))
	}
}

func isPoliteClose(code int) bool {
	switch code {
	case websocket.CloseGoingAway:
		return true
	case websocket.CloseNormalClosure:
		return true
	default:
		return false
	}
}
