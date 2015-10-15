package common

import "time"

// Device represents a generic LIFX device
type Device interface {
	// Returns the ID for the device
	ID() uint64

	// GetLabel gets the label for the device
	GetLabel() (string, error)
	// SetLabel sets the label for the device
	SetLabel(label string) error
	// GetPower requests the current power state of the device, true for on,
	// false for off
	GetPower() (bool, error)
	// CachedPower returns the last known power state of the device, true for
	// on, false for off
	CachedPower() bool
	// SetPower sets the power state of the device, true for on, false for off
	SetPower(state bool) error

	// Device is a SubscriptionTarget
	SubscriptionTarget
}

// Light represents a LIFX light device
type Light interface {
	// SetColor changes the color of the light, transitioning over the specified
	// duration
	SetColor(color Color, duration time.Duration) error
	// GetColor requests the current color of the light
	GetColor() (Color, error)
	// CachedColor returns the last known color of the light
	CachedColor() Color
	// SetPowerDuration sets the power of the light, transitioning over the
	// speficied duration, state is true for on, false for off.
	SetPowerDuration(state bool, duration time.Duration) error

	// Light is a superset of the Device interface
	Device
}
