// Copyright 2013-2017 Apcera Inc. All rights reserved.

package wsconn

import (
	"io"
	"io/ioutil"
	"net"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Returns a websocket connection wrapper to the net.Conn interface.
func NewWebsocketConnection(ws Conn) net.Conn {
	wsconn := &WebsocketConnection{
		ws:         ws,
		closedChan: make(chan bool),
		textChan:   make(chan []byte, 100),
	}
	ws.SetPingHandler(makePingHandler(ws, &wsconn.writeMutex))
	ws.SetPongHandler(makePongHandler(ws))
	ws.SetCloseHandler(func(code int, msg string) error {
		wsconn.writeMutex.Lock()
		defer wsconn.writeMutex.Unlock()
		wsconn.closeCode = code
		return ws.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(code, ""),
			time.Now().Add(defaultWriteTimeout))
	})
	wsconn.startPingInterval()
	return wsconn
}

// WebsocketConnection is a wrapper around a websocket connect from a lower
// level API.  It supports things such as automatic ping/pong keepalive.
type WebsocketConnection struct {
	ws         Conn
	reader     io.Reader
	writeMutex sync.Mutex
	closedChan chan bool
	textChan   chan []byte
	closeCode  int
}

// Begins a goroutine to send a periodic ping to the other end
func (conn *WebsocketConnection) startPingInterval() {
	go func() {
		// Set the initial read timeout
		conn.ws.SetReadDeadline(time.Now().Add(defaultReadTimeout))
		for {
			select {
			case <-conn.closedChan:
				return
			case <-time.After(defaultPingInterval):
				func() {
					conn.writeMutex.Lock()
					defer conn.writeMutex.Unlock()
					conn.ws.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(defaultWriteTimeout))
				}()
			}
		}
	}()
}

// This method loops until a binary opcode comes in and returns its reader.
// While looping it will receive and process other opcodes, such as pings.
func (conn *WebsocketConnection) waitForReader() error {
	// no existing readers, wait for one
	for {
		opCode, reader, err := conn.ws.NextReader()
		if err != nil {
			// Gorilla Websockets 1.2 no longer emits websocket.CloseMessage
			// from NextReader(), and instead registers a close handler. If that
			// close handler was called conn.closeCode will have the value.
			conn.writeMutex.Lock()
			defer conn.writeMutex.Unlock()
			if conn.closeCode != 0 {
				return io.EOF
			}
			return err
		}

		switch opCode {
		case websocket.BinaryMessage:
			// binary packet
			conn.reader = reader
			return nil

		case websocket.TextMessage:
			// plain text package
			b, err := ioutil.ReadAll(reader)
			if err == nil {
				conn.textChan <- b
			}
		}
	}
}

// GetTextChannel returns a channel outputting all text messages from the
// websocket.
func (conn *WebsocketConnection) GetTextChannel() <-chan []byte {
	return conn.textChan
}

// Reads slice of bytes off of the websocket connection.
func (conn *WebsocketConnection) Read(b []byte) (n int, err error) {
	if conn.reader == nil {
		err = conn.waitForReader()
		if err != nil {
			return
		}
	}

	rn, rerr := conn.reader.Read(b)
	switch rerr {
	case io.EOF:
		conn.reader = nil
	default:
		n, err = rn, rerr
	}
	return
}

// Writes the given bytes as a binary opcode segment onto the websocket.
func (conn *WebsocketConnection) Write(b []byte) (n int, err error) {
	conn.writeMutex.Lock()
	defer conn.writeMutex.Unlock()

	// allocate a writer
	var writer io.WriteCloser
	writer, err = conn.ws.NextWriter(websocket.BinaryMessage)
	if err != nil {
		return
	}

	// write
	n, err = writer.Write(b)
	if err != nil {
		return
	}

	// close it
	err = writer.Close()
	return
}

// Closes the connection and exits from the ping loop.
func (conn *WebsocketConnection) Close() error {
	defer close(conn.closedChan)
	defer close(conn.textChan)
	return conn.ws.Close()
}

// LocalAddr returns the local net.Addr of the websocket connection.
func (conn *WebsocketConnection) LocalAddr() net.Addr {
	return conn.ws.LocalAddr()
}

// RemoteAddr returns the remote net.Addr of the websocket connection.
func (conn *WebsocketConnection) RemoteAddr() net.Addr {
	return conn.ws.RemoteAddr()
}

// SetDeadline the read and write deadlines associated with the connection.
func (conn *WebsocketConnection) SetDeadline(t time.Time) error {
	if err := conn.ws.SetReadDeadline(t); err != nil {
		return err
	}
	return conn.ws.SetWriteDeadline(t)
}

// SetReadDeadline sets the read deadline associated with the connection.
func (conn *WebsocketConnection) SetReadDeadline(t time.Time) error {
	return conn.ws.SetReadDeadline(t)
}

// SetWriteDeadline sets the write deadline assocated with the connection.
func (conn *WebsocketConnection) SetWriteDeadline(t time.Time) error {
	return conn.ws.SetWriteDeadline(t)
}
