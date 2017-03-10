// Copyright 2013 Apcera Inc. All rights reserved.

package wsconn

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"testing"

	"github.com/gorilla/websocket"
)

type wsTestServer struct {
	*testing.T

	readChan  chan string
	writeChan chan string
}

func (t wsTestServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", 405)
		t.Logf("Bad method: %s", r.Method)
		return
	}

	if m, _ := regexp.MatchString("http://127\\.0\\.0\\.1:\\d+/endpoint", r.Header.Get("Origin")); !m {
		http.Error(w, "Origin not allowed", 403)
		t.Logf("Bad origin: %s", r.Header.Get("Origin"))
		return
	}

	// upgrade the connection to use websockets
	ws, err := websocket.Upgrade(w, r, nil, 1024, 1024)
	if err != nil {
		http.Error(w, "Error in upgrade", 500)
		t.Logf("Error when upgrading: %v", err)
		return
	}

	wsconn := NewWebsocketConnection(ws)
	defer wsconn.Close()
	finishedChan := make(chan bool)

	go func() {
		for {
			b := make([]byte, 1024)
			n, err := wsconn.Read(b)
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				finishedChan <- true
				return
			} else if n == 0 && err == nil {
				continue
			} else if err != nil {
				t.Logf("Read error: %v", err)
				finishedChan <- true
				return
			}
			t.readChan <- string(b[:n])
		}
	}()

	go func() {
		for {
			str := <-t.writeChan
			if str == "" {
				t.Logf("Received blank string, shutting down")
				finishedChan <- true
				return
			}

			_, err := wsconn.Write([]byte(str))
			if err != nil {
				t.Logf("Write error: %v", err)
				finishedChan <- true
				return
			}
		}
	}()

	<-finishedChan
}

func TestWebsocketConnection(t *testing.T) {
	handler := wsTestServer{t, make(chan string), make(chan string)}
	server := httptest.NewServer(handler)
	defer server.Close()

	// parse the url
	nurl, err := url.ParseRequestURI(server.URL)
	if err != nil {
		t.Fatalf("url.ParseRequestURI returned an error: %v", err)
	}
	nurl.Path = "/endpoint"

	// intialize headers
	headers := http.Header{
		"Origin": {nurl.String()},
	}

	// connect to the server
	conn, err := net.Dial("tcp", nurl.Host)
	if err != nil {
		t.Fatalf("net.Dial returned an error: %v", err)
	}

	// initialize the wesocket client
	ws, _, err := websocket.NewClient(conn, nurl, headers, 1024, 1024)
	if err != nil {
		t.Fatalf("websocket.NewClient returned an error: %v", err)
	}

	// create the connection
	wsconn := NewWebsocketConnection(ws)
	defer wsconn.Close()

	// helper functions
	testRead := func(str string) {
		// write it
		_, err := wsconn.Write([]byte(str))
		if err != nil {
			t.Fatalf("Write error in test: %v", err)
		}

		// wait for it on the channel
		act := <-handler.readChan

		// test it
		if str != act {
			t.Fatalf("Read failed\nExpected: %q\nActual: %q", str, act)
		}
	}

	testWrite := func(str string) {
		// push the string to write onto the channel
		handler.writeChan <- str

		// loop, sometimes get back a 0 and nil error
		var act string
		for {
			b := make([]byte, 1024)
			n, err := wsconn.Read(b)
			if n == 0 && err == nil {
				continue
			} else if err != nil {
				t.Fatalf("Read error in test: %v", err)
				return
			}
			act = string(b[:n])
			break
		}

		// test it
		if str != act {
			t.Fatalf("Write failed\nExpected: %q\nActual: %q", str, act)
			return
		}
	}

	// verify a read
	testRead("echo")
	testRead("foobar")
	testWrite("something else")
	testWrite("another string")
	testRead("one last read")
	testWrite("Another write!")
}
