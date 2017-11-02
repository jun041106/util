// Copyright 2017 Apcera Inc. All rights reserved.

package wsconn

import (
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// Multiplexer provides for sending, and receiving an arbitrary number
// (actually 2^32) of discrete streams / datagrams over a single websocket
// connection.  It implements a simple protocol over the websocket:
// - All frames are prefixed with a 4-byte network order uint32 channel
//   ID header. The remainder of the frame is the data from a single
//   write().
// - A stream or datagram channel can be marked closed by sending an
//   empty message (just the 4-byte channel ID header)
type Multiplexer struct {
	ws Conn
	// inStreams maps channelID to stream-style destinations for data coming in
	// from the network.
	inStreams map[uint32]io.WriteCloser
	// inStreamErrors maps channelID to an error channel where we can report any
	// errors that happen writing to the user provided destinations.
	inStreamErrors map[uint32]chan error
	// inMessages maps channelID to user-provided chan<- []byte where we will
	// publish any messages received with the specified channelID.
	inMessages      map[uint32]chan<- []byte
	outStreams      map[uint32]io.ReadCloser
	outStreamErrors map[uint32]chan error
	// remoteClosed lists all channels the remote side has reported closed.
	remoteClosed map[uint32]bool
	mutex        sync.Mutex
	err          error
	readerWG     sync.WaitGroup
	writerWG     sync.WaitGroup
	readTimeout  time.Duration
	writeTimeout time.Duration
	pingInterval time.Duration
	sentClose    bool
	sendCloser   chan struct{}
	closed       bool
}

type mWriter struct {
	m         *Multiplexer
	channelID uint32
	closed    bool
}

type nopWriteCloser struct {
	io.Writer
}

// Close is a no-op which allows an io.Writer to implement io.WriteCloser
func (nopWriteCloser) Close() error {
	return nil
}

// NopWriteCloser adapts an io.Writer into a io.WriteCloser, by adding a no-op
// Close() method. Similar to ioutil.NopCloser, but for writers instead readers.
func NopWriteCloser(w io.Writer) io.WriteCloser {
	return nopWriteCloser{Writer: w}
}

// NewMultiplexer creates a properly initialized Multiplexer.
func NewMultiplexer(ws Conn) *Multiplexer {
	m := &Multiplexer{
		ws:              ws,
		sendCloser:      make(chan struct{}),
		inStreams:       make(map[uint32]io.WriteCloser),
		inStreamErrors:  make(map[uint32]chan error),
		inMessages:      make(map[uint32]chan<- []byte),
		outStreams:      make(map[uint32]io.ReadCloser),
		outStreamErrors: make(map[uint32]chan error),
		remoteClosed:    make(map[uint32]bool),
		readTimeout:     defaultReadTimeout,
		writeTimeout:    defaultWriteTimeout,
		pingInterval:    defaultPingInterval,
	}
	return m
}

// copy works just like io.Copy, but provides attribution if any error occurs.
func copy(dst io.Writer, src io.Reader, buf []byte) (n uint64, dstErr, srcErr error) {
	var written uint64
	var nr int
	var nw int
	for {
		nr, srcErr = src.Read(buf)
		if srcErr != nil {
			if srcErr == io.EOF {
				// This is the success case.
				return written, nil, nil
			}
			return written, nil, srcErr
		}
		if nr > 0 {
			nw, dstErr = dst.Write(buf[0:nr])
			if nw > 0 {
				written += uint64(nw)
			}
			if dstErr != nil {
				return written, dstErr, nil
			}
			if nw != nr {
				// This behavior mimics io.Copy, but if short write errors are
				// seen in the wild it may be worth adding retry logic here.
				return written, io.ErrShortWrite, nil
			}
		}

	}
}

// HandleInboundStream will copy any inbound (from net) stream data tagged with
// `channelID` to the provided `dest` writer. Upon receipt of an EOF from the
// remote on `channelID` or close of the Multiplexer, `dest` will be closed.
// Any errors encountered writing to `dest` will be sent on the returned
// <-chan error, and it will be closed. The returned <-chan error will be closed
// upon closure of this channelID or closure of the Multiplexer. If `dest` does
// not implement Closer or close-on-eof is not desired, consider using
// `wsconn.NopWriteCloser{Writer:myWriter}`.
func (m *Multiplexer) HandleInboundStream(channelID uint32, dest io.WriteCloser) (<-chan error, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if _, ok := m.inStreams[channelID]; ok {
		return nil, fmt.Errorf("channel id %d already registered", channelID)
	}
	m.inStreams[channelID] = dest
	errChan := make(chan error)
	m.inStreamErrors[channelID] = errChan
	return errChan, nil
}

// HandleInboundMessage will send any messages received with `channelID` to the
// provided (`dest`) channel.
func (m *Multiplexer) HandleInboundMessage(channelID uint32, dest chan<- []byte) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.inMessages[channelID] = dest
}

// AddOutboundStream takes an io.ReadCloser (or io.Reader wrapped in
// a ioutil.NopCloser) and copies it to the specified channel on the
// websocket. If the `r` encounters an EOF, a close frame will be
// transmitted to the remote. If the remote closes the reader or the
// websocket transfer layer is closed, `r`` will be Closed.
func (m *Multiplexer) AddOutboundStream(channelID uint32, r io.ReadCloser) <-chan error {
	m.writerWG.Add(1)
	errChan := make(chan error)
	go func() {
		defer m.writerWG.Done()
		defer close(errChan)
		w := m.GetStreamWriter(channelID)
		defer w.Close()
		m.mutex.Lock()
		m.outStreams[channelID] = r
		m.outStreamErrors[channelID] = errChan
		m.mutex.Unlock()
		defer func() {
			m.mutex.Lock()
			defer m.mutex.Unlock()
			m.closeOutboundStream(channelID)
		}()
		buf := make([]byte, 32*1024)
		_, srcErr, dstErr := copy(w, r, buf)

		func() {
			m.mutex.Lock()
			defer m.mutex.Unlock()
			// The error channel may have been removed if the reader was
			// intentionally closed. Load it fresh from m.outStreamErrors so we can
			// detect this.
			if errChanNow, ok := m.outStreamErrors[channelID]; ok {
				if srcErr != nil {
					errChanNow <- srcErr
				}
			}

			// Always remove the error channel.
			delete(m.outStreamErrors, channelID)
		}()

		if dstErr != nil {
			m.close(dstErr)
		}
	}()

	return errChan
}

// closeOutboundStream is called on all open outbound streams before SendClose()
// and also called by AddOutboundStream to cleanup after it completes.
func (m *Multiplexer) closeOutboundStream(channelID uint32) {
	if r, ok := m.outStreams[channelID]; ok {
		// We're about to force AddOutboundStream to read from a closed reader
		// there will be a src error, delete the channel so it can't report it.
		delete(m.outStreamErrors, channelID)
		delete(m.outStreams, channelID)
		// Ignore close errors.
		r.Close()
	}
}

// Close shuts down the underlying connection and closes all local destination
// writers.
func (m *Multiplexer) Close() error {
	m.close(nil)
	m.readerWG.Wait()
	m.writerWG.Wait()
	return m.err
}

// SendClose notifies the remote we are done sending data. We may continue to
// read data from the remote.
func (m *Multiplexer) SendClose(closeCode int, text string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.sentClose {
		return websocket.ErrCloseSent
	}
	err := m.sendClose(closeCode, text)
	// Because of vendoring, we can't reliably compare to websocket.ErrCloseSent
	if err != nil && !strings.Contains(err.Error(), "websocket: close sent") {
		m.close(err)
	}
	return m.err
}

func (m *Multiplexer) sendClose(closeCode int, text string) error {
	if m.sentClose {
		return nil
	}
	m.sentClose = true
	// This will interrupt any internal send routines (like pinger)
	close(m.sendCloser)
	// Close all outbound streams. It's an error to write after sending close.
	for channelID := range m.outStreamErrors {
		m.closeOutboundStream(channelID)
	}
	return m.ws.WriteControl(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(closeCode, text),
		time.Now().Add(m.writeTimeout))
}

func (m *Multiplexer) closeInboundSources() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	for channelID, stream := range m.inStreams {
		// We promised to close the destination stream. Do that, report error to
		// the destination error channel, and then close that channel.
		err := stream.Close()
		if errChan, ok := m.inStreamErrors[channelID]; ok {
			if err != nil {
				m.inStreamErrors[channelID] <- err
			}
			close(errChan)
			delete(m.inStreamErrors, channelID)
		}
		delete(m.inStreams, channelID)
	}
	for channelID, msgChan := range m.inMessages {
		close(msgChan)
		delete(m.inMessages, channelID)
	}
}

func (m *Multiplexer) close(err error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.closed {
		return
	}
	m.closed = true
	if err != nil && m.err == nil {
		m.err = err
	}
	// we promised to close all outbound streams.
	for channelID := range m.outStreamErrors {
		m.closeOutboundStream(channelID)
	}
	if err != nil {
		m.err = err
		err = m.sendClose(websocket.CloseInternalServerErr, "IO Error")
	} else {
		err = m.sendClose(websocket.CloseNormalClosure, "Closed")
	}
	if err != nil && m.err == nil {
		m.err = err
	}
	err = m.ws.Close()
	if err != nil && m.err == nil {
		m.err = err
	}
}

// Start begins distributing messages/streams and then immediately returns.
func (m *Multiplexer) Start() {
	m.writerWG.Add(1)
	go m.pinger()
	m.readerWG.Add(1)
	go m.reader()
}

// WaitForRemote blocks until the remote side indicates its done sending data.
// If WaitForRemote returns no error, then the remote side should still be
// accepting new data and the callers should take this opportunity to finish
// sending, and politely close the connection.
func (m *Multiplexer) WaitForRemote() error {
	m.readerWG.Wait()
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.err
}

func (m *Multiplexer) pinger() {
	defer m.writerWG.Done()
	m.ws.SetReadDeadline(time.Now().Add(m.readTimeout))
	for {
		select {
		case <-m.sendCloser:
			return
		case <-time.After(m.pingInterval):
			func() {
				m.mutex.Lock()
				defer m.mutex.Unlock()
				m.ws.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(m.writeTimeout))
			}()
		}
	}
}

func (m *Multiplexer) reader() {
	defer m.readerWG.Done()
	defer m.closeInboundSources()
	sBuf := make([]byte, 4)
	copyBuf := make([]byte, 32*1024)

	m.ws.SetPingHandler(makePingHandler(m.ws, &m.mutex))
	m.ws.SetPongHandler(makePongHandler(m.ws))

	var closeCode int32
	// Override the default close handler which immediately sends a close. The
	// app is responsible for seeing that we were closed with
	// Multiplexer.WaitForRemote(). NextReader will return websocket.CloseError
	// immediately after the close handler returns. We save the code here
	// because the CloseError is opaque to use since it's a struct from the
	// caller's vendored websocket library.
	m.ws.SetCloseHandler(func(code int, text string) error {
		atomic.StoreInt32(&closeCode, int32(code))
		return nil
	})

	for {
		opCode, reader, err := m.ws.NextReader()
		if err != nil {
			// io.EOF means the read-side of the network socket is done, but
			// the remote may still be accepting data, close out the inbound
			// readers let our side finish writing data, and then close
			// politely.
			if !isPoliteClose(int(atomic.LoadInt32(&closeCode))) && err != io.EOF {
				m.close(err)
			}
			return
		}

		if opCode != websocket.BinaryMessage {
			m.close(fmt.Errorf("unexpected websocket message type: %d", opCode))
			return
		}

		// We need 4 bytes for the sentinel
		_, err = io.ReadFull(reader, sBuf)
		if err != nil {
			m.close(fmt.Errorf(
				"incomplete read of multiplex sentinel: %v", err))
			return
		}

		// Decode the sentinel. Network order uint32
		channelID := binary.BigEndian.Uint32(sBuf)

		m.mutex.Lock()
		stream, isStream := m.inStreams[channelID]
		messageChan, isMessage := m.inMessages[channelID]
		m.mutex.Unlock()
		if isStream {
			n, dstErr, srcErr := copy(stream, reader, copyBuf)
			if srcErr != nil {
				// This error happened on the websocket side of the stream.
				// It's not an EOF, so the connection is likely broken.
				m.close(srcErr)
				return
			} else if dstErr != nil {
				func() {
					m.mutex.Lock()
					defer m.mutex.Unlock()
					// The writer we were given errored out. This could happen
					// if for example the other end (maybe a program) closed the
					// stream. In this case it's worth closing this stream, but
					// not worth blowing up the other channels.
					if errChan, ok := m.inStreamErrors[channelID]; ok {
						errChan <- dstErr
						close(errChan)
						delete(m.inStreamErrors, channelID)
					}
					// We promised to close the inbound writer. Ignore errors.
					stream.Close()
					// Once an error occurs we shouldn't try to write again,
					// replace the real writer with a discard writer. We're
					// going to let the sender know we're closed for business,
					// but there may be some packets on the wire.
					m.inStreams[channelID] = NopWriteCloser(ioutil.Discard)
					// Write an empty message on the same channel to let the
					// sender know we're closed
				}()

				err = m.CloseMessageChannel(channelID)
				if err != nil && err != io.ErrClosedPipe {
					m.close(srcErr)
					return
				}
			} else if n == 0 {
				// If the remote closes the connection, we close the writer
				// and remove it from our list. It should be an error to
				// see any more messages with this channelID
				m.mutex.Lock()
				dstErr := stream.Close()
				if errChan, ok := m.inStreamErrors[channelID]; ok {
					if dstErr != nil {
						errChan <- dstErr
					}
					close(errChan)
					delete(m.inStreamErrors, channelID)
				}
				delete(m.inStreams, channelID)
				m.mutex.Unlock()
			}
		} else if isMessage {
			msg, err := ioutil.ReadAll(reader)
			if err != nil { // This is always an src IO error
				m.close(err)
				return
			} else if len(msg) == 0 {
				// See above note about closing the connection.
				m.mutex.Lock()
				close(messageChan)
				delete(m.inMessages, channelID)
				m.mutex.Unlock()
			}
			messageChan <- msg
		} else {
			n, err := io.Copy(ioutil.Discard, reader)
			if err != nil && n == 0 {
				// The remote sent this to indicate its no longer accepting
				// data on this channel.
				m.mutex.Lock()
				m.remoteClosed[channelID] = true
				m.mutex.Unlock()
			} else {
				m.close(fmt.Errorf("data received for unregistered channelID %d", channelID))
				return
			}

		}

	}
}

// WriteMessage sends `msg` to the remote using `channelID`. Unlike using
// m.GetStreamWriter().Write(), WriteMessage provides message semantics instead
// of stream semantics.
func (m *Multiplexer) WriteMessage(channelID uint32, msg []byte) (n int, err error) {
	defer func() {
		// Report the write error, but do it outside the mutex lock.
		if err != nil && err != io.ErrClosedPipe {
			m.close(err)
		}
	}()
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.sentClose || m.remoteClosed[channelID] {
		return 0, io.ErrClosedPipe
	}
	// If this is a close message, record it
	if len(msg) == 0 {
		m.remoteClosed[channelID] = true
	}
	w, err := m.ws.NextWriter(websocket.BinaryMessage)
	if err != nil {
		return 0, err
	}
	defer func() {
		e := w.Close()
		// Report close errors (that's where the actual network IO lives), but
		// only if there's not already an earlier error.
		if err == nil {
			err = e
		}
	}()
	sentinel := make([]byte, 4)
	binary.BigEndian.PutUint32(sentinel, channelID)
	_, err = w.Write(sentinel)
	if err != nil {
		return 0, err
	}
	return w.Write(msg)
}

// CloseMessageChannel indicates to the remote that we will no longer be
// accepting reads on this channel, or will no longer be writing to this channel.
func (m *Multiplexer) CloseMessageChannel(channelID uint32) error {
	_, err := m.WriteMessage(channelID, []byte{})
	return err
}

// GetStreamWriter opens a write channel on the multiplexer & returns
// an io.WriteCloser.
func (m *Multiplexer) GetStreamWriter(channelID uint32) io.WriteCloser {
	return &mWriter{
		m:         m,
		channelID: channelID,
	}
}

// Write implements io.Writer.  It transmits `data` in stream format
// on the channel specified in `Multiplexer.GetStreamWriter`.
func (w *mWriter) Write(data []byte) (int, error) {
	if w.closed {
		return 0, io.ErrClosedPipe
	}
	n, err := w.m.WriteMessage(w.channelID, data)
	if err != nil {
		w.closed = true
	}
	return n, err
}

// Close implements io.Closer.  It transmits a close notification
// on the channel specified in `Multiplexer.GetStreamWriter`.
func (w *mWriter) Close() error {
	w.closed = true
	err := w.m.CloseMessageChannel(w.channelID)
	if err == io.ErrClosedPipe {
		return nil
	}
	return err
}
