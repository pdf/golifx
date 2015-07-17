package common

import "time"

// Device represents a generic LIFX device
type Device interface {
	// Returns the ID for the device
	ID() uint64

	// Returns the label for the device
	GetLabel() (string, error)
	// Sets the label for the device
	SetLabel(label string) error

	// Returns the power state of the device, true for on, false for off
	GetPower() (bool, error)
	// Sets the power state of the device, true for on, false for off
	SetPower(state bool) error
}

// Light represents a LIFX light device
type Light interface {
	// A light is a superset of the Device interface
	Device
	// SetColor changes the color of the light, over the specified duration
	SetColor(color Color, duration time.Duration) error
	// GetColor returns the current color of the light
	GetColor() (Color, error)
}
