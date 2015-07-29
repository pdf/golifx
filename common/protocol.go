package common

import "time"

// Protocol defines the interface between the Client and a protocol
// implementation
type Protocol interface {
	SubscriptionTarget
	// SetClient sets the client on the protocol for bi-directional
	// communication
	SetClient(client Client)
	// Discover initiates device discovery, this may be a noop in some future
	// protocol versions.  This is called immediately when the client connects
	// to the protocol
	Discover() error
	// Close closes the protocol driver, no further communication with the
	// protocol is possible
	Close() error
	// NewSubscription returns a *Subscription for a Client to obtain
	// events from the Protocol

	// SetPower sets the power state globally, on all devices
	SetPower(state bool) error
	// SetPowerDuration sets the power state globally, on all lights, over the
	// specified duration
	SetPowerDuration(state bool, duration time.Duration) error
	// SetColor changes the color globally, on all lights, over the specified
	// duration
	SetColor(color Color, duration time.Duration) error
}
