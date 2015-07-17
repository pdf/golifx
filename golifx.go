// Copyright 2015 Peter Fern
// Use of this source code is governed by the MIT
// license that can be found in the LICENSE file

// Package golifx provides a simple Go interface to the LIFX LAN protocol.
//
// Also included in cmd/lifx is a small CLI utility that allows interacting with
// your LIFX devices on the LAN.
package golifx

import (
	"time"

	"github.com/pdf/golifx/common"
	"github.com/pdf/golifx/protocol"
)

const (
	// VERSION of this library
	VERSION = `0.0.1`
)

// NewClient returns a pointer to a new Client and any error that occurred
// initializing the client, using the protocol p.  It also kicks off a discovery
// run.
func NewClient(p protocol.Protocol) (*Client, error) {
	c := &Client{
		protocol:              p,
		devices:               make(map[uint64]common.Device),
		timeout:               common.DefaultTimeout,
		retryInterval:         common.DefaultRetryInterval,
		internalRetryInterval: 10 * time.Millisecond,
		quitChan:              make(chan bool, 1),
	}
	p.SetClient(c)
	err := c.discover()
	return c, err
}

// SetLogger allows assigning a custom levelled logger that conforms to the
// common.Logger interface.  To capture logs generated during client creation,
// this should be called before creating a Client. Defaults to
// common.StubLogger, which does no logging at all.
func SetLogger(logger common.Logger) {
	common.SetLogger(logger)
}
