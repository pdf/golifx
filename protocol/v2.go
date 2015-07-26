package protocol

import (
	"net"
	"sync"
	"time"

	"github.com/pdf/golifx/common"
	"github.com/pdf/golifx/protocol/v2/device"
	"github.com/pdf/golifx/protocol/v2/packet"
	"github.com/pdf/golifx/protocol/v2/shared"
)

// V2 implements the LIFX LAN protocol version 2.
type V2 struct {
	// Port determines UDP port for this protocol instance
	Port          int
	initialized   bool
	socket        *net.UDPConn
	client        common.Client
	timeout       *time.Duration
	retryInterval *time.Duration
	broadcast     *device.Light
	lastDiscovery time.Time
	devices       map[uint64]device.GenericDevice
	quitChan      chan bool
	sync.RWMutex
}

// SetClient sets the client on the protocol for bi-directional communication
func (p *V2) SetClient(client common.Client) {
	p.client = client
	p.timeout = client.GetTimeout()
	p.retryInterval = client.GetRetryInterval()
}

func (p *V2) init() error {
	p.Lock()
	defer p.Unlock()
	if !p.initialized {
		socket, err := net.ListenUDP(`udp4`, &net.UDPAddr{Port: shared.DefaultPort})
		if err != nil {
			return err
		}
		p.socket = socket
		addr := net.UDPAddr{
			IP:   net.IPv4(255, 255, 255, 255),
			Port: shared.DefaultPort,
		}
		dev, err := device.New(&addr, p.socket, p.timeout, p.retryInterval, nil)
		if err != nil {
			return err
		}
		p.broadcast = &device.Light{Device: *dev}
		p.devices = make(map[uint64]device.GenericDevice)
		p.quitChan = make(chan bool, 1)
		go p.dispatcher()
		p.initialized = true
	}
	return nil
}

// Discover initiates device discovery, this may be a noop in some future
// protocol versions.  This is called immediately when the client connects to
// the protocol
func (p *V2) Discover() error {
	p.RLock()
	initialized := p.initialized
	p.RUnlock()
	if !initialized {
		if err := p.init(); err != nil {
			return err
		}
	}
	if p.lastDiscovery.After(time.Time{}) {
		var extinct []uint64
		p.RLock()
		for _, dev := range p.devices {
			// If the device has not been seen in twice the time since the last
			// discovery, mark it as extinct
			if dev.Seen().Before(time.Now().Add(time.Now().Sub(p.lastDiscovery) * -2)) {
				extinct = append(extinct, dev.ID())
			}
		}
		p.RUnlock()
		// Remove extinct devices
		for _, id := range extinct {
			p.Lock()
			delete(p.devices, id)
			p.Unlock()
			err := p.client.RemoveDeviceByID(id)
			if err != nil {
				common.Log.Warnf("Failed removing extinct device '%d' from client: %v", id, err)
			}
		}
	}
	p.broadcast.Discover()
	p.Lock()
	p.lastDiscovery = time.Now()
	p.Unlock()

	return nil
}

// SetPower sets the power state globally, on all devices
func (p *V2) SetPower(state bool) error {
	return p.broadcast.SetPower(state)
}

// SetPower sets the power state globally, on all devices
func (p *V2) SetPowerDuration(state bool, duration time.Duration) error {
	return p.broadcast.SetPowerDuration(state, duration)
}

// SetColor changes the color globally, on all lights, over the specified
// duration
func (p *V2) SetColor(color common.Color, duration time.Duration) error {
	return p.broadcast.SetColor(color, duration)
}

// Close closes the protocol driver, no further communication with the protocol
// is possible
func (p *V2) Close() error {
	p.quitChan <- true
	return nil
}

func (p *V2) dispatcher() {
	for {
		select {
		case <-p.quitChan:
			p.Lock()
			for _, dev := range p.devices {
				err := dev.Close()
				if err != nil {
					common.Log.Errorf("Failed closing device '%v': %v\n", dev.ID(), err)
				}
			}
			p.Unlock()
			return
		default:
			buf := make([]byte, 1500)
			n, addr, err := p.socket.ReadFromUDP(buf)
			if err != nil {
				common.Log.Fatalf("Failed reading from socket: %v\n", err)
			}
			pkt, err := packet.Decode(buf[:n])
			if err != nil {
				common.Log.Fatalf("Failed decoding packet: %v\n", err)
			}
			go p.process(pkt, addr)
		}
	}
}

func (p *V2) getDevice(id uint64) (device.GenericDevice, error) {
	p.RLock()
	dev, ok := p.devices[id]
	p.RUnlock()
	if !ok {
		return nil, common.ErrNotFound
	}

	return dev, nil
}

func (p *V2) process(pkt *packet.Packet, addr *net.UDPAddr) {
	common.Log.Debugf("Processing packet from %v: source %v, type %v, sequence %v, target %v, tagged %v, resRequired %v, ackRequired %v: %+v\n", addr.IP, pkt.GetSource(), pkt.GetType(), pkt.GetSequence(), pkt.GetTarget(), pkt.GetTagged(), pkt.GetResRequired(), pkt.GetAckRequired(), *pkt)
	if pkt.Target != 0 {
		dev, err := p.getDevice(pkt.Target)
		if err == nil {
			dev.SetSeen(time.Now())
		}
	}
	if pkt.GetSource() != packet.ClientID {
		switch pkt.GetType() {
		case device.StatePower:
			dev, err := p.getDevice(pkt.GetTarget())
			if err != nil {
				common.Log.Debugf("Skipping StatePower packet for unknown device: source %v, type %v, sequence %v, target %v, tagged %v, resRequired %v, ackRequired %v: %+v\n", pkt.GetSource(), pkt.GetType(), pkt.GetSequence(), pkt.GetTarget(), pkt.GetTagged(), pkt.GetResRequired(), pkt.GetAckRequired(), *pkt)
				return
			}
			err = dev.SetStatePower(pkt)
			if err != nil {
				common.Log.Debugf("Failed setting StatePower on device: source %v, type %v, sequence %v, target %v, tagged %v, resRequired %v, ackRequired %v: %+v\n", pkt.GetSource(), pkt.GetType(), pkt.GetSequence(), pkt.GetTarget(), pkt.GetTagged(), pkt.GetResRequired(), pkt.GetAckRequired(), *pkt)
				return
			}
		case device.StateLabel:
			dev, err := p.getDevice(pkt.GetTarget())
			if err != nil {
				common.Log.Debugf("Skipping StateLabel packet for unknown device: source %v, type %v, sequence %v, target %v, tagged %v, resRequired %v, ackRequired %v: %+v\n", pkt.GetSource(), pkt.GetType(), pkt.GetSequence(), pkt.GetTarget(), pkt.GetTagged(), pkt.GetResRequired(), pkt.GetAckRequired(), *pkt)
				return
			}
			dev.SetStateLabel(pkt)
			if err != nil {
				common.Log.Debugf("Failed setting StatePower on device: source %v, type %v, sequence %v, target %v, tagged %v, resRequired %v, ackRequired %v: %+v\n", pkt.GetSource(), pkt.GetType(), pkt.GetSequence(), pkt.GetTarget(), pkt.GetTagged(), pkt.GetResRequired(), pkt.GetAckRequired(), *pkt)
				return
			}
		case device.State:
			dev, err := p.getDevice(pkt.GetTarget())
			if err != nil {
				common.Log.Debugf("Skipping State packet for unknown device: source %v, type %v, sequence %v, target %v, tagged %v, resRequired %v, ackRequired %v: %+v\n", pkt.GetSource(), pkt.GetType(), pkt.GetSequence(), pkt.GetTarget(), pkt.GetTagged(), pkt.GetResRequired(), pkt.GetAckRequired(), *pkt)
				return
			}
			light, ok := dev.(*device.Light)
			if !ok {
				common.Log.Debugf("Skipping State packet for non-light device: source %v, type %v, sequence %v, target %v, tagged %v, resRequired %v, ackRequired %v: %+v\n", pkt.GetSource(), pkt.GetType(), pkt.GetSequence(), pkt.GetTarget(), pkt.GetTagged(), pkt.GetResRequired(), pkt.GetAckRequired(), *pkt)
				return
			}
			err = light.SetState(pkt)
			if err != nil {
				common.Log.Debugf("Error setting State on device: source %v, type %v, sequence %v, target %v, tagged %v, resRequired %v, ackRequired %v: %+v\n", pkt.GetSource(), pkt.GetType(), pkt.GetSequence(), pkt.GetTarget(), pkt.GetTagged(), pkt.GetResRequired(), pkt.GetAckRequired(), *pkt)
				return
			}
		default:
			common.Log.Debugf("Skipping packet with non-local source: source %v, type %v, sequence %v, target %v, tagged %v, resRequired %v, ackRequired %v: %+v\n", pkt.GetSource(), pkt.GetType(), pkt.GetSequence(), pkt.GetTarget(), pkt.GetTagged(), pkt.GetResRequired(), pkt.GetAckRequired(), *pkt)
		}
		return
	}
	switch pkt.GetType() {
	case device.StateService:
		if _, err := p.getDevice(pkt.Target); err != nil {
			dev, err := device.New(addr, p.socket, p.timeout, p.retryInterval, pkt)
			if err != nil {
				common.Log.Errorf("Failed creating device: %v\n", err)
				return
			}
			p.addDevice(dev)
		}
	default:
		if pkt.GetTarget() == 0 {
			common.Log.Debugf("Skipping packet without target: %+v\n", *pkt)
			return
		}
		dev, err := p.getDevice(pkt.GetTarget())
		if err != nil {
			common.Log.Errorf("No known device with ID %v\n", pkt.GetTarget())
			return
		}
		dev.Handle(pkt)
	}
}

func (p *V2) addDevice(dev *device.Device) {
	common.Log.Debugf("Attempting to add device: %v\n", dev.ID())
	_, err := p.getDevice(dev.ID())
	if err == nil {
		common.Log.Debugf("Device already known: %v\n", dev.ID())
		return
	}
	p.Lock()
	p.devices[dev.ID()] = dev
	p.Unlock()
	vendor, err := dev.GetHardwareVendor()
	if err != nil {
		common.Log.Errorf("Error retrieving device hardware vendor: %v\n", err)
		return
	}
	product, err := dev.GetHardwareProduct()
	if err != nil {
		common.Log.Errorf("Error retrieving device hardware product: %v\n", err)
		return
	}
	if vendor == device.VendorLifx && product == device.ProductLifxOriginal {
		p.Lock()
		// Need to figure if there's a way to do this without being racey on the
		// lock inside the dev
		l := &device.Light{Device: *dev}
		p.devices[l.ID()] = l
		p.Unlock()
		common.Log.Debugf("New device is a light: %v\n", l.ID())
		if err := l.Get(); err != nil {
			common.Log.Debugf("Failed getting light state: %v\n", err)
		}
		common.Log.Debugf("Adding device to client: %v\n", l.ID())
		if err := p.client.AddDevice(l); err != nil {
			if err == common.ErrDuplicate {
				common.Log.Debugf("Device exists on client: %v\n", l.ID())
			} else {
				common.Log.Errorf("Error adding device to client: %v\n", err)
			}
			return
		}
	} else {
		common.Log.Debugf("Adding device to client: %v\n", dev.ID())
		if err := p.client.AddDevice(dev); err != nil {
			if err == common.ErrDuplicate {
				common.Log.Debugf("Device exists on client: %v\n", dev.ID())
			} else {
				common.Log.Errorf("Error adding device to client: %v\n", err)
			}
			return
		}
	}
	common.Log.Debugf("Added device to client: %v\n", dev.ID())
}
