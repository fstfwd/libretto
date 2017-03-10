// Copyright 2013 Apcera Inc. All rights reserved.

package wsconn

import (
	"io"
	"io/ioutil"
	"net"
	"sync"
	"time"

	"github.com/gorilla/websocket"
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

	Close() error
}

// Returns a websocket connection wrapper to the net.Conn interface.
func NewWebsocketConnection(ws Conn) net.Conn {
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

// WebsocketConnection is a wrapper around a websocket connect from a lower
// level API.  It supports things such as automatic ping/pong keepalive.
type WebsocketConnection struct {
	ws           Conn
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
					conn.ws.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(conn.writeTimeout))
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

		case websocket.PingMessage:
			// receeived a ping, so send a pong
			go func() {
				conn.writeMutex.Lock()
				defer conn.writeMutex.Unlock()
				conn.ws.WriteControl(websocket.PongMessage, []byte{}, time.Now().Add(conn.writeTimeout))
			}()

		case websocket.PongMessage:
			// received a pong, update read deadline
			conn.ws.SetReadDeadline(time.Now().Add(conn.readTimeout))

		case websocket.CloseMessage:
			// received close, so return EOF
			return io.EOF
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
