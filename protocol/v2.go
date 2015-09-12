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
	Port int
	// Reliable enables reliable comms, requests ACKs for all operations to
	// ensure they're delivered (recommended)
	Reliable      bool
	initialized   bool
	socket        *net.UDPConn
	client        common.Client
	timeout       *time.Duration
	retryInterval *time.Duration
	broadcast     *device.Light
	lastDiscovery time.Time
	devices       map[uint64]device.GenericDevice
	subscriptions map[string]*common.Subscription
	locations     map[string]*device.Location
	groups        map[string]*device.Group
	quitChan      chan bool
	sync.RWMutex
}

// SetClient sets the client on the protocol for bi-directional communication
func (p *V2) SetClient(client common.Client) {
	p.timeout = client.GetTimeout()
	p.retryInterval = client.GetRetryInterval()
}

// NewSubscription returns a new *common.Subscription for receiving events from
// this protocol.
func (p *V2) NewSubscription() (*common.Subscription, error) {
	if err := p.init(); err != nil {
		return nil, err
	}
	sub := common.NewSubscription(p)
	p.Lock()
	p.subscriptions[sub.ID()] = sub
	p.Unlock()
	return sub, nil
}

// CloseSubscription is a callback for handling the closing of subscriptions.
func (p *V2) CloseSubscription(sub *common.Subscription) error {
	p.RLock()
	_, ok := p.subscriptions[sub.ID()]
	p.RUnlock()
	if !ok {
		return common.ErrNotFound
	}
	p.Lock()
	delete(p.subscriptions, sub.ID())
	p.Unlock()

	return nil
}

func (p *V2) init() error {
	p.RLock()
	if p.initialized {
		p.RUnlock()
		return nil
	}
	p.RUnlock()

	p.Lock()
	defer p.Unlock()
	if p.Port == 0 {
		p.Port = shared.DefaultPort
	}
	socket, err := net.ListenUDP(`udp4`, &net.UDPAddr{Port: p.Port})
	if err != nil {
		return err
	}
	p.socket = socket
	addr := net.UDPAddr{
		IP:   net.IPv4(255, 255, 255, 255),
		Port: p.Port,
	}
	broadcastDev, err := device.New(&addr, p.socket, p.timeout, p.retryInterval, false, nil)
	if err != nil {
		return err
	}
	p.broadcast = &device.Light{Device: *broadcastDev}
	p.devices = make(map[uint64]device.GenericDevice)
	p.locations = make(map[string]*device.Location)
	p.groups = make(map[string]*device.Group)
	p.subscriptions = make(map[string]*common.Subscription)
	p.quitChan = make(chan bool, 1)
	go p.dispatcher()
	p.initialized = true

	return nil
}

// Pushes an event to subscribers
func (p *V2) publish(event interface{}) error {
	p.RLock()
	subs := make(map[string]*common.Subscription, len(p.subscriptions))
	for k, sub := range p.subscriptions {
		subs[k] = sub
	}
	p.RUnlock()

	for _, sub := range subs {
		if err := sub.Write(event); err != nil {
			return err
		}
	}

	return nil
}

// Discover initiates device discovery, this may be a noop in some future
// protocol versions.  This is called immediately when the client connects to
// the protocol
func (p *V2) Discover() error {
	if err := p.init(); err != nil {
		return err
	}
	if p.lastDiscovery.After(time.Time{}) {
		var extinct []device.GenericDevice
		p.RLock()
		for _, dev := range p.devices {
			// If the device has not been seen in twice the time since the last
			// discovery, mark it as extinct
			if dev.Seen().Before(time.Now().Add(time.Now().Sub(p.lastDiscovery) * -2)) {
				extinct = append(extinct, dev)
			}
		}
		p.RUnlock()
		// Remove extinct devices
		for _, dev := range extinct {
			p.Lock()
			delete(p.devices, dev.ID())
			p.Unlock()

			locationID, err := dev.GetLocation()
			if err == nil {
				p.RLock()
				location, ok := p.locations[locationID]
				p.RUnlock()
				if ok {
					if err = location.RemoveDevice(dev); err != nil {
						common.Log.Warnf("Failed removing extinct device '%d' from location (%v): %v", dev.ID(), locationID, err)
					}
					if len(location.Devices()) == 0 {
						p.publish(common.EventExpiredLocation{Location: location})
					}
				}
			}

			groupID, err := dev.GetGroup()
			if err == nil {
				p.RLock()
				group, ok := p.groups[groupID]
				p.RUnlock()
				if ok {
					if err = group.RemoveDevice(dev); err != nil {
						common.Log.Warnf("Failed removing extinct device '%d' from group (%v): %v", dev.ID(), groupID, err)
					}
					if len(group.Devices()) == 0 {
						p.publish(common.EventExpiredGroup{Group: group})
					}
				}
			}

			err = p.publish(common.EventExpiredDevice{Device: dev})
			if err != nil {
				common.Log.Warnf("Failed removing extinct device '%d' from client: %v", dev.ID(), err)
			}
		}
	}
	if err := p.broadcast.Discover(); err != nil {
		return err
	}
	p.Lock()
	p.lastDiscovery = time.Now()
	p.Unlock()

	return nil
}

// SetPower sets the power state globally, on all devices
func (p *V2) SetPower(state bool) error {
	return p.broadcast.SetPower(state)
}

// SetPowerDuration sets the power state globally, on all devices, transitioning
// over the specified duration
func (p *V2) SetPowerDuration(state bool, duration time.Duration) error {
	return p.broadcast.SetPowerDuration(state, duration)
}

// SetColor changes the color globally, on all lights, transitioning over the
// specified duration
func (p *V2) SetColor(color common.Color, duration time.Duration) error {
	return p.broadcast.SetColor(color, duration)
}

// Close closes the protocol driver, no further communication with the protocol
// is possible
func (p *V2) Close() error {
	for _, sub := range p.subscriptions {
		if err := sub.Close(); err != nil {
			return err
		}
	}

	p.Lock()
	defer p.Unlock()

	for _, location := range p.locations {
		if err := location.Close(); err != nil {
			return err
		}
	}

	for _, group := range p.groups {
		if err := group.Close(); err != nil {
			return err
		}
	}

	for _, dev := range p.devices {
		if err := dev.Close(); err != nil {
			return err
		}
	}

	select {
	case <-p.quitChan:
		common.Log.Warnf(`protocol already closed`)
		return common.ErrClosed
	default:
		close(p.quitChan)
	}

	return nil
}

func (p *V2) dispatcher() {
	for {
		select {
		case <-p.quitChan:
			p.Lock()
			for _, dev := range p.devices {
				if err := dev.Close(); err != nil {
					common.Log.Errorf("Failed closing device '%v': %v\n", dev.ID(), err)
				}
			}
			if err := p.socket.Close(); err != nil {
				common.Log.Errorf("Failed closing socket: %v\n", err)
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

	// Update device seen time for any targeted packets
	if pkt.Target != 0 {
		dev, err := p.getDevice(pkt.Target)
		if err == nil {
			dev.SetSeen(time.Now())
		}
	}

	// Broadcast packets, or packets generated by other clients
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
			err = dev.SetStateLabel(pkt)
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

	// Packets processed at the protocol level regardless of target
	switch pkt.GetType() {
	case device.StateLocation:
		p.addLocation(pkt)
	case device.StateGroup:
		p.addGroup(pkt)
	}

	// Packets processed at the protocol level or returned to target
	switch pkt.GetType() {
	case device.StateService:
		dev, err := p.getDevice(pkt.Target)
		if err != nil {
			dev, err := device.New(addr, p.socket, p.timeout, p.retryInterval, p.Reliable, pkt)
			if err != nil {
				common.Log.Errorf("Failed creating device: %v\n", err)
				return
			}
			p.addDevice(dev)
			return
		}
		// Perform state discovery on lights
		if l, ok := dev.(*device.Light); ok {
			if err := l.Get(); err != nil {
				common.Log.Debugf("Failed getting light state: %v\n", err)
			}
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
		common.Log.Debugf("Returning packet to device %v: %+v\n", dev.ID(), *pkt)
		dev.Handle(pkt)
	}
}

func (p *V2) addLocation(pkt *packet.Packet) {
	l, err := device.NewLocation(pkt)
	if err != nil {
		common.Log.Errorf("Error parsing location: %v\n", err)
		return
	}
	p.RLock()
	location, ok := p.locations[l.ID()]
	p.RUnlock()
	if !ok {
		p.Lock()
		p.locations[l.ID()] = l
		p.Unlock()
		if err := p.publish(common.EventNewLocation{Location: l}); err != nil {
			common.Log.Errorf("Error adding location to client: %v\n", err)
			return
		}
	} else {
		if err := location.Parse(pkt); err != nil {
			common.Log.Errorf("Error parsing location: %v\n", err)
		}
	}
}

func (p *V2) addGroup(pkt *packet.Packet) {
	g, err := device.NewGroup(pkt)
	if err != nil {
		common.Log.Errorf("Error parsing group: %v\n", err)
		return
	}
	p.RLock()
	group, ok := p.groups[g.ID()]
	p.RUnlock()
	if !ok {
		p.Lock()
		p.groups[g.ID()] = g
		p.Unlock()
		if err := p.publish(common.EventNewGroup{Group: g}); err != nil {
			common.Log.Errorf("Error adding group to client: %v\n", err)
			return
		}
	} else {
		if err := group.Parse(pkt); err != nil {
			common.Log.Errorf("Error parsing group: %v\n", err)
		}
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

	locationID, err := dev.GetLocation()
	if err != nil {
		common.Log.Warnf("Error retrieving device location: %v\n", err)
	}
	p.RLock()
	location, ok := p.locations[locationID]
	p.RUnlock()
	if !ok {
		common.Log.Warnf("Unknown location ID: %v\n", locationID)
	}

	groupID, err := dev.GetGroup()
	if err != nil {
		common.Log.Warnf("Error retrieving device group: %v\n", err)
	}
	p.RLock()
	group, ok := p.groups[groupID]
	p.RUnlock()
	if !ok {
		common.Log.Warnf("Unknown group ID: %v\n", groupID)
	}

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
	if vendor == device.VendorLifx {
		switch product {
		case device.ProductLifxOriginal, device.ProductLifxColor650, device.ProductLifxWhite800:
			p.Lock()
			// Need to figure if there's a way to do this without being racey on the
			// lock inside the dev
			l := &device.Light{Device: *dev}
			p.devices[l.ID()] = l
			p.Unlock()
			common.Log.Debugf("New device is a light: %v\n", l.ID())
			common.Log.Debugf("Adding device to location (%s): %v\n", locationID, l.ID())
			if err := location.AddDevice(l); err != nil {
				common.Log.Warnf("Error adding device to location: %v\n", err)
			}
			common.Log.Debugf("Adding device to group (%s): %v\n", groupID, l.ID())
			if err := group.AddDevice(l); err != nil {
				common.Log.Warnf("Error adding device to group: %v\n", err)
			}
			if err := l.Get(); err != nil {
				common.Log.Debugf("Failed getting light state: %v\n", err)
			}
			common.Log.Debugf("Adding device to client: %v\n", l.ID())
			if err := p.publish(common.EventNewDevice{Device: l}); err != nil {
				common.Log.Errorf("Error adding device to client: %v\n", err)
				return
			}
			common.Log.Debugf("Added device to client: %v\n", dev.ID())
			// Lights get an early return because we've set everything up for
			// them, other devices are handled generically below
			return
		default:
			common.Log.Debugf("Adding device to client: %v\n", dev.ID())
			if err := p.publish(common.EventNewDevice{Device: dev}); err != nil {
				common.Log.Errorf("Error adding device to client: %v\n", err)
				return
			}
		}
	} else {
		common.Log.Debugf("Adding device to client: %v\n", dev.ID())
		if err := p.publish(common.EventNewDevice{Device: dev}); err != nil {
			common.Log.Errorf("Error adding device to client: %v\n", err)
		}
	}
	common.Log.Debugf("Adding device to location (%s): %v\n", locationID, dev.ID())
	if err := location.AddDevice(dev); err != nil {
		common.Log.Warnf("Error adding device to location: %v\n", err)
	}
	common.Log.Debugf("Adding device to group (%s): %v\n", groupID, dev.ID())
	if err := group.AddDevice(dev); err != nil {
		common.Log.Warnf("Error adding device to group: %v\n", err)
	}
	common.Log.Debugf("Added device to client: %v\n", dev.ID())
}
