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
	*Device
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

	if l.color != s.Color {
		l.Lock()
		l.color = s.Color
		l.Unlock()
		if err := l.publish(common.EventUpdateColor{Color: l.color}); err != nil {
			return err
		}
	}
	if l.power != s.Power {
		l.Lock()
		l.power = s.Power
		l.Unlock()
		if err := l.publish(common.EventUpdatePower{Power: l.power > 0}); err != nil {
			return err
		}
	}
	newLabel := stripNull(string(s.Label[:]))
	if newLabel != l.label {
		l.Lock()
		l.label = newLabel
		l.Unlock()
		if err := l.publish(common.EventUpdateLabel{Label: l.label}); err != nil {
			return err
		}
	}

	return nil
}

func (l *Light) Get() error {
	pkt := packet.New(l.address, l.requestSocket)
	pkt.SetType(Get)
	req, err := l.Send(pkt, l.reliable, true)
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
	if err := pkt.SetPayload(p); err != nil {
		return err
	}
	req, err := l.Send(pkt, l.reliable, false)
	if err != nil {
		return err
	}
	if l.reliable {
		// Wait for ack
		<-req
		common.Log.Debugf("Setting color on %v acknowledged\n", l.id)
	}

	l.color = color
	if err := l.publish(common.EventUpdateColor{Color: l.color}); err != nil {
		return err
	}

	return nil
}

func (l *Light) GetColor() (common.Color, error) {
	if err := l.Get(); err != nil {
		return common.Color{}, err
	}
	return l.CachedColor(), nil
}

func (l *Light) CachedColor() common.Color {
	l.RLock()
	color := l.color
	l.RUnlock()
	return color
}

func (l *Light) SetPowerDuration(state bool, duration time.Duration) error {
	if state && l.power > 0 {
		return nil
	}

	p := new(payloadPowerDuration)
	if state {
		p.Level = math.MaxUint16
	}
	p.Duration = uint32(duration / time.Millisecond)

	pkt := packet.New(l.address, l.requestSocket)
	pkt.SetType(LightSetPower)
	if err := pkt.SetPayload(p); err != nil {
		return err
	}

	common.Log.Debugf("Setting power state on %v: %v\n", l.id, state)
	req, err := l.Send(pkt, l.reliable, false)
	if err != nil {
		return err
	}
	if l.reliable {
		// Wait for ack
		<-req
		common.Log.Debugf("Setting power state on %v acknowledged\n", l.id)
	}

	l.Lock()
	l.power = p.Level
	l.Unlock()
	if err := l.publish(common.EventUpdatePower{Power: l.power > 0}); err != nil {
		return err
	}

	return nil
}
