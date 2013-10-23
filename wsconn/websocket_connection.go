// Copyright 2013 Apcera Inc. All rights reserved.

package wsconn

import (
	"io"
	"io/ioutil"
	"net"
	"sync"
	"time"

	"github.com/apcera/cntm-deps/go-websocket/websocket"
)

// Returns a websocket connection wrapper to the net.Conn interface.
func NewWebsocketConnection(ws *websocket.Conn) net.Conn {
	wsconn := &WebsocketConnection{
		ws:           ws,
		readTimeout:  60 * time.Second,
		writeTimeout: 10 * time.Second,
		pingInterval: 10 * time.Second,
		closedChan:   make(chan bool),
		textChan:     make(chan []byte, 100),
	}
	wsconn.startPingInterval()
	return wsconn
}

type WebsocketConnection struct {
	ws           *websocket.Conn
	reader       io.Reader
	writeMutex   sync.Mutex
	readTimeout  time.Duration
	writeTimeout time.Duration
	pingInterval time.Duration
	closedChan   chan bool
	textChan     chan []byte
}

// Begins a goroutine to send a periodic ping to the other end
func (conn *WebsocketConnection) startPingInterval() {
	go func() {
		for {
			select {
			case <-conn.closedChan:
				return
			case <-time.After(conn.pingInterval):
				func() {
					conn.writeMutex.Lock()
					defer conn.writeMutex.Unlock()
					err := conn.ws.WriteControl(websocket.OpPong, []byte{}, time.Now().Add(conn.writeTimeout))
					if err != nil {
						Log.Errorf("Error writing websocket ping: %v", err)
					}
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
			return err
		}

		switch opCode {
		case websocket.OpBinary:
			// binary packet
			conn.reader = reader
			return nil

		case websocket.OpText:
			// plain text package
			b, err := ioutil.ReadAll(reader)
			if err == nil {
				conn.textChan <- b
			}

		case websocket.OpPing:
			// receeived a ping, so send a pong
			go func() {
				conn.writeMutex.Lock()
				defer conn.writeMutex.Unlock()
				err := conn.ws.WriteControl(websocket.OpPong, []byte{}, time.Now().Add(conn.writeTimeout))
				if err != nil {
					Log.Errorf("Error writing websocket ping: %v", err)
				}
			}()

		case websocket.OpPong:
			// received a pong, update read deadline
			err := conn.ws.SetReadDeadline(time.Now().Add(conn.readTimeout))
			if err != nil {
				Log.Errorf("Error writing websocket pong: %v", err)
			}

		case websocket.OpClose:
			// received close, so return EOF
			return io.EOF
		}
	}
}

func (conn *WebsocketConnection) GetTextChannel() <-chan []byte {
	return conn.textChan
}

// Reads a slice of bytes off of the websocket connection.
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
	writer, err = conn.ws.NextWriter(websocket.OpBinary)
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

// Returns the local portion of the connection.
func (conn *WebsocketConnection) LocalAddr() net.Addr {
	return conn.ws.LocalAddr()
}

// Returns the remote endpoint for the connection.
func (conn *WebsocketConnection) RemoteAddr() net.Addr {
	return conn.ws.RemoteAddr()
}

// Sets the read and write deadlines associated with the connection.
func (conn *WebsocketConnection) SetDeadline(t time.Time) error {
	if err := conn.ws.SetReadDeadline(t); err != nil {
		return err
	}
	return conn.ws.SetWriteDeadline(t)
}

// Sets the read deadline associated with the connection.
func (conn *WebsocketConnection) SetReadDeadline(t time.Time) error {
	return conn.ws.SetReadDeadline(t)
}

// Sets the write deadline assocated with the connection.
func (conn *WebsocketConnection) SetWriteDeadline(t time.Time) error {
	return conn.ws.SetWriteDeadline(t)
}
