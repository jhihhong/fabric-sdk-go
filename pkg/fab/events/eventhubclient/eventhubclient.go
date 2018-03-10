/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package eventhubclient

import (
	"time"

	"github.com/hyperledger/fabric-sdk-go/pkg/common/context"

	"github.com/hyperledger/fabric-sdk-go/pkg/context/api/fab"
	"github.com/hyperledger/fabric-sdk-go/pkg/fab/events/api"
	"github.com/hyperledger/fabric-sdk-go/pkg/fab/events/client"
	"github.com/hyperledger/fabric-sdk-go/pkg/fab/events/eventhubclient/connection"

	"github.com/hyperledger/fabric-sdk-go/pkg/common/options"
	"github.com/hyperledger/fabric-sdk-go/pkg/fab/events/eventhubclient/dispatcher"
	"github.com/hyperledger/fabric-sdk-go/pkg/logging"
	"github.com/pkg/errors"
)

var logger = logging.NewLogger("fabsdk/fab")

var ehConnProvider = func(context context.Client, chConfig fab.ChannelCfg, peer fab.Peer) (api.Connection, error) {
	eventEndpoint, ok := peer.(api.EventEndpoint)
	if !ok {
		panic("peer is not an EventEndpoint")
	}

	return connection.New(
		context, chConfig, eventEndpoint.EventURL(),
	)
}

// Client connects to a peer and receives channel events, such as bock, filtered block, chaincode, and transaction status events.
type Client struct {
	client.Client
	params
}

// New returns a new event hub client
func New(context context.Client, chConfig fab.ChannelCfg, opts ...options.Opt) (*Client, error) {
	params := defaultParams()
	options.Apply(params, opts)

	// The EventHub requires a custom Discovery Provider
	// that produces EventEndpoints (which include the event URL).
	ehCtx := newEventHubContext(context)

	client := &Client{
		Client: *client.New(
			params.permitBlockEvents,
			dispatcher.New(ehCtx, chConfig, params.connProvider, opts...),
			opts...,
		),
		params: *params,
	}
	client.SetAfterConnectHandler(client.registerInterests)

	if err := client.Start(); err != nil {
		return nil, err
	}

	return client, nil
}

func (c *Client) registerInterests() error {
	logger.Debugf("sending register interests request....")

	errch := make(chan error)
	c.Submit(dispatcher.NewRegisterInterestsEvent(c.interests, errch))

	var err error
	select {
	case err = <-errch:
	case <-time.After(c.respTimeout):
		err = errors.New("timeout waiting for register interests response")
	}

	if err != nil {
		logger.Errorf("unable to send register interests request: %s", err)
		return err
	}

	logger.Debugf("successfully sent register interests")
	return nil
}

// ehContext overrides the DiscoveryProvider by returning
// the event hub discovery provider
type ehContext struct {
	context.Client
}

func newEventHubContext(ctx context.Client) context.Client {
	return &ehContext{
		Client: ctx,
	}
}

// DiscoveryProvider returns a custom discovery provider for the event hub
func (ctx *ehContext) DiscoveryProvider() fab.DiscoveryProvider {
	return newDiscoveryProvider(ctx.Client)
}