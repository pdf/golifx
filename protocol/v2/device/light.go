package device

import (
	"math"
	"time"

	"github.com/pdf/golifx/common"
	"github.com/pdf/golifx/protocol/v2/packet"
	"github.com/pdf/golifx/protocol/v2/shared"
)

const (
	Get             shared.Message = 101
	SetColor        shared.Message = 102
	State           shared.Message = 107
	LightGetPower   shared.Message = 116
	LightSetPower   shared.Message = 117
	LightStatePower shared.Message = 118
)

type Light struct {
	Device
	color common.Color
}

type payloadColor struct {
	Reserved uint8
	Color    common.Color
	Duration uint32
}

type payloadPowerDuration struct {
	Level    uint16
	Duration uint32
}

type state struct {
	Color     common.Color
	Reserved0 int16
	Power     uint16
	Label     [32]byte
	Reserved1 uint64
}

func (l *Light) SetState(pkt *packet.Packet) error {
	s := &state{}

	if err := pkt.DecodePayload(s); err != nil {
		return err
	}
	common.Log.Debugf("Got light state (%v): %+v\n", l.id, s)

	l.color = s.Color
	l.power = s.Power
	l.label = stripNull(string(s.Label[:]))

	return nil
}

func (l *Light) Get() error {
	pkt := packet.New(l.address, l.requestSocket)
	pkt.SetType(Get)
	req, err := l.Send(pkt, false, true)
	if err != nil {
		return err
	}

	common.Log.Debugf("Waiting for light state (%v)\n", l.id)
	pktResponse := <-req
	if pktResponse.Error != nil {
		return pktResponse.Error
	}

	err = l.SetState(&pktResponse.Result)
	if err != nil {
		return err
	}

	return nil
}

func (l *Light) SetColor(color common.Color, duration time.Duration) error {
	if l.color == color {
		return nil
	}
	if duration < shared.RateLimit {
		duration = shared.RateLimit
	}
	p := &payloadColor{
		Color:    color,
		Duration: uint32(duration / time.Millisecond),
	}

	pkt := packet.New(l.address, l.requestSocket)
	pkt.SetType(SetColor)
	pkt.SetPayload(p)
	_, err := l.Send(pkt, false, false)
	if err != nil {
		return err
	}
	l.color = color
	return nil
}

func (l *Light) GetColor() (common.Color, error) {
	return l.color, nil
}

func (l *Light) SetPowerDuration(state bool, duration time.Duration) error {
	p := new(payloadPowerDuration)
	if state {
		p.Level = math.MaxUint16
	}
	if l.power == p.Level {
		return nil
	}
	p.Duration = uint32(duration / time.Millisecond)

	pkt := packet.New(l.address, l.requestSocket)
	pkt.SetType(LightSetPower)
	pkt.SetPayload(p)
	if _, err := l.Send(pkt, false, false); err != nil {
		return err
	}

	l.power = p.Level

	return nil
}
