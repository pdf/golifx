package device

import (
	"time"

	"github.com/pdf/golifx/common"
	"github.com/pdf/golifx/protocol/v2/packet"
)

type GenericDevice interface {
	common.Device
	Handle(*packet.Packet)
	Close() error
	Seen() time.Time
	SetSeen(time.Time)
	SetStatePower(*packet.Packet) error
	SetStateLabel(*packet.Packet) error
	SetStateLocation(*packet.Packet) error
	SetStateGroup(*packet.Packet) error
	GetLocation() (string, error)
	GetGroup() (string, error)
	GetHardwareVendor() (uint32, error)
	GetHardwareProduct() (uint32, error)
	ResetLimiter()
}
