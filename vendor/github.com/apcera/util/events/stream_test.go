// Copyright 2016 Apcera Inc. All right reserved.

package events

import (
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"testing"
	"time"

	turnpike "gopkg.in/jcelliott/turnpike.v2"
)

const (
	eventsRealm = "com.apcera.api.es"
)

func TestWAMPClient_NoEventsSent(t *testing.T) {
	port, _, l := newTestWebsocketServer(t)
	defer l.Close()

	wampClient, err := NewWAMPSessionClient(fmt.Sprintf("wss://localhost:%d", port), "", eventsRealm)
	if err != nil {
		t.Fatalf("Failed to create WAMP client: %s", err)
	}

	if err := wampClient.StreamEvents(ioutil.Discard, "job::/prod", 50*time.Millisecond); err != nil {
		t.Fatalf("Failed to stream events: %s", err)
	}
}

// newTestWebsocketServer returns a new started websocket listener. This code
// (and dependent functions) is lifted from `turnpike`.
func newTestWebsocketServer(t *testing.T) (int, turnpike.Router, io.Closer) {
	s, err := turnpike.NewWebsocketServer(map[string]turnpike.Realm{
		eventsRealm: turnpike.Realm{},
	})
	if err != nil {
		t.Fatalf("Failed to create websocket server: %s", err)
	}
	server := &http.Server{
		Handler: s,
	}

	var addr net.TCPAddr
	l, err := net.ListenTCP("tcp", &addr)
	if err != nil {
		t.Fatalf("Failed to listen: %s", err)
	}
	go server.Serve(l)
	return l.Addr().(*net.TCPAddr).Port, s.Router, l
}
