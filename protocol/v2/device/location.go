package device

import "github.com/pdf/golifx/protocol/v2/packet"

type Location struct {
	// Location is a Group
	Group
}

func NewLocation(pkt *packet.Packet) (*Location, error) {
	l := new(Location)
	l.init()
	if err := l.Parse(pkt); err != nil {
		return l, err
	}

	return l, nil
}
