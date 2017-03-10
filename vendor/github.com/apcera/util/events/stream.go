// Copyright 2016 Apcera Inc. All right reserved.

package events

import (
	"fmt"
	"io"
	"time"

	turnpike "gopkg.in/jcelliott/turnpike.v2"
)

// StreamEvents streams events from a given resource FQN to a specific writer.
// It blocks until the stream is disconnected, and will continue writing to
// the given writer. It will time if it has not received an event within the
// given timeout.
func (e *EventClient) StreamEvents(w io.Writer, streamFQN string, timeout time.Duration) error {
	timeoutTicker := time.NewTimer(timeout).C

	handleEventStream := func(args []interface{}, kwargs map[string]interface{}) {
		event := args[0].(map[string]interface{})

		output := fmt.Sprintf("FQN: %q, Source: %s, Time: %b, Type: %d, Payload: %v", event["resource"], event["event_source"], event["time"], int(event["type"].(float64)), event["payload"])
		fmt.Fprintln(w, output)
		timeoutTicker = time.NewTimer(timeout).C
	}

	if err := e.Subscribe(streamFQN, handleEventStream); err != nil {
		return err
	}

	// By default, nginx terminates inactive connections after 60 seconds. So,
	// we pong.
	keepAlive := time.NewTicker(30 * time.Second)
	defer keepAlive.Stop()
	for {
		select {
		// Streams till the connection is closed by the API Server
		case <-e.ReceiveDone:
			return nil
		// If the connection has remained inactive beyond input.Timeout, unsubscribe
		case <-timeoutTicker:
			return nil
		// NGINX terminates inactive connections post 60 seconds.
		// Sending a WAMP Hello{} message (harmless) as a packet ping to NGINX
		case <-keepAlive.C:
			e.Send(&turnpike.Hello{})
		}
	}
}
