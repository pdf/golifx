package common

import "time"

// Client defines the interface required by protocols
type Client interface {
	AddDevice(Device) error
	RemoveDeviceByID(uint64) error
	GetTimeout() *time.Duration
	GetRetryInterval() *time.Duration
}
