// Copyright 2016 Apcera Inc. All right reserved.

package events

import (
	"net/url"

	turnpike "gopkg.in/jcelliott/turnpike.v2"
)

// An EventClient allows for streaming of events.
type EventClient struct {
	*turnpike.Client
}

// NewWAMPSessionClient returns a client handle associated with a session. It
// accepts an optional authorization token to supply as a query parameter on the
// initial request.
func NewWAMPSessionClient(wampServerURL, authToken, realm string) (*EventClient, error) {
	u, err := url.Parse(wampServerURL)
	if err != nil {
		return nil, err
	}

	if u.Scheme == "https" {
		u.Scheme = "wss"
	} else {
		u.Scheme = "ws"
	}

	targetURL := u.String()
	if authToken != "" {
		authorization, err := url.Parse(authToken)
		if err != nil {
			return nil, err
		}

		if authorization.String() != "" {
			query := "authorization=" + authorization.String()
			targetURL += "?" + query
		}
	}

	wampClient, err := turnpike.NewWebsocketClient(turnpike.JSON, targetURL, nil)
	if err != nil {
		return nil, err
	}

	_, err = wampClient.JoinRealm(realm, nil)
	if err != nil {
		return nil, err
	}
	// ReceiveDone is notified when the client's connection to the router is lost.
	wampClient.ReceiveDone = make(chan bool)

	return &EventClient{wampClient}, nil
}
