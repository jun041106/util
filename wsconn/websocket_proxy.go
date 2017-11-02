package wsconn

import (
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Proxy takes two websocket connections and copies all messages and controls
// (except ping) between each connection. It maintains a ping on each connection
// end.
func Proxy(a Conn, b Conn) error {
	var wg sync.WaitGroup
	errors := make(chan error, 4)

	var aMutex sync.Mutex
	var bMutex sync.Mutex
	var aClosed bool
	var bClosed bool

	a.SetReadDeadline(time.Now().Add(defaultReadTimeout))
	b.SetReadDeadline(time.Now().Add(defaultReadTimeout))

	copyIO := func(
		src Conn, srcMutex *sync.Mutex,
		dst Conn, dstMutex *sync.Mutex, dstClosed *bool,
	) (err error) {
		defer wg.Done()
		// Cheat and use this defer to report our exit status, and to close out
		// politely tell the dst we're done sending data.
		defer func() {
			dstMutex.Lock()
			defer dstMutex.Unlock()
			*dstClosed = true
			switch err {
			case nil:
				// We copied the remote's closing message.
			case io.EOF:
				// The src hung-up without a close. Be more polite and give the
				// dest a close message. It's possible the source is half-open
				// and still accepting data, so don't blow up.
				err = dst.WriteControl(
					websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseGoingAway, "EOF"),
					time.Now().Add(5*time.Second))
			default:
				// Some nasty error happened and its not clear if it happened on
				// the src or dst connections. Try to let the dst know we're
				// bailing, but preserve the original error.
				dst.WriteControl(
					websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseGoingAway, "Error"),
					time.Now().Add(5*time.Second))
			}
			errors <- err
		}()

		pingHandler := makePingHandler(src, srcMutex)
		pongHandler := makePongHandler(src)
		src.SetPingHandler(pingHandler)
		src.SetPongHandler(pongHandler)
		var closeCode int
		src.SetCloseHandler(func(code int, msg string) error {
			dstMutex.Lock()
			defer dstMutex.Unlock()
			closeCode = code
			*dstClosed = true
			return dst.WriteControl(
				websocket.CloseMessage,
				websocket.FormatCloseMessage(code, msg),
				time.Now().Add(defaultWriteTimeout))
		})

		for {
			opCode, reader, err := src.NextReader()
			if err != nil {
				if isPoliteClose(closeCode) {
					return nil
				}
				return err
			}

			if opCode != websocket.BinaryMessage && opCode != websocket.TextMessage {
				return fmt.Errorf("unexpected websocket message type: %d", opCode)
			}

			err = func() error {
				dstMutex.Lock()
				defer dstMutex.Unlock()
				w, err := dst.NextWriter(opCode)
				if err != nil {
					return err
				}
				defer w.Close()
				_, err = io.Copy(w, reader)
				return err
			}()
			if err != nil {
				return err
			}
		}
	}

	pingClosed := make(chan struct{})

	go func() {
		for {
			select {
			case <-pingClosed:
				return
			case <-time.After(defaultPingInterval):
				func() {
					aMutex.Lock()
					defer aMutex.Unlock()
					if !aClosed {
						err := a.WriteControl(
							websocket.PingMessage,
							[]byte{},
							time.Now().Add(defaultWriteTimeout))
						if err != nil {
							aClosed = true
							errors <- err
						}
					}
				}()
				func() {
					bMutex.Lock()
					defer bMutex.Unlock()
					if !bClosed {
						err := b.WriteControl(
							websocket.PingMessage,
							[]byte{},
							time.Now().Add(defaultWriteTimeout))
						if err != nil {
							bClosed = true
							errors <- err
						}
					}
				}()
			}
		}
	}()

	wg.Add(2)
	go copyIO(a, &aMutex, b, &bMutex, &bClosed)
	go copyIO(b, &bMutex, a, &aMutex, &aClosed)
	wg.Wait()
	close(pingClosed)
	close(errors) // Both writers have quit and waited for, this is safe.

	// Will return the first error (EOF doesn't count) or nil.
	for {
		err, ok := <-errors
		if !ok || err == io.EOF {
			return nil
		}
		return err
	}
}
