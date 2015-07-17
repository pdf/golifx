// Package protocol implements the LIFX LAN protocol.
//
// This package is not designed to used directly by end users, other than to
// specify a protocol version when creating a new Client from the golifx
// package.
//
// The currently implemented protocol versions are:
//   V2
package protocol

import (
	"time"

	"github.com/pdf/golifx/common"
)

// Protocol defines the interface between the Client and a protocol
// implementation
type Protocol interface {
	// SetClient sets the client on the protocol for bi-directional
	// communication
	SetClient(client common.Client)
	// Discover initiates device discovery, this may be a noop in some future
	// protocol versions.  This is called immediately when the client connects
	// to the protocol
	Discover() error
	// Close closes the protocol driver, no further communication with the
	// protocol is possible
	Close() error

	// SetPower sets the power state globally, on all devices
	SetPower(state bool) error
	// SetColor changes the color globally, on all lights, over the specified
	// duration
	SetColor(color common.Color, duration time.Duration) error
}
