// Package device implements a LIFX LAN protocol version 2 device.
//
// This package is not designed to be accessed by end users, all interaction
// should occur via the Client in the golifx package.
package device

import (
	"math"
	"net"
	"sync"
	"time"

	"github.com/pdf/golifx/common"
	"github.com/pdf/golifx/protocol/v2/packet"
	"github.com/pdf/golifx/protocol/v2/shared"
)

const (
	GetService        shared.Message = 2
	StateService      shared.Message = 3
	GetHostInfo       shared.Message = 12
	StateHostInfo     shared.Message = 13
	GetHostFirmware   shared.Message = 14
	StateHostFirmware shared.Message = 15
	GetWifiInfo       shared.Message = 16
	StateWifiInfo     shared.Message = 17
	GetWifiFirmware   shared.Message = 18
	StateWifiFirmware shared.Message = 19
	GetPower          shared.Message = 20
	SetPower          shared.Message = 21
	StatePower        shared.Message = 22
	GetLabel          shared.Message = 23
	SetLabel          shared.Message = 24
	StateLabel        shared.Message = 25
	GetVersion        shared.Message = 32
	StateVersion      shared.Message = 33
	GetInfo           shared.Message = 34
	StateInfo         shared.Message = 35
	Acknowledgement   shared.Message = 45
	GetLocation       shared.Message = 48
	StateLocation     shared.Message = 50
	GetGroup          shared.Message = 51
	StateGroup        shared.Message = 53
	EchoRequest       shared.Message = 58
	EchoResponse      shared.Message = 59

	VendorLifx = 1

	ProductLifxOriginal1000        uint32 = 1
	ProductLifxColor650            uint32 = 3
	ProductLifxWhite800LowVoltage  uint32 = 10
	ProductLifxWhite800HighVoltage uint32 = 11
	ProductLifxColor1000           uint32 = 22
)

type responseMap map[uint8]packet.Chan
type doneMap map[uint8]chan struct{}

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
	ResetLimiter()
}

type Device struct {
	id              uint64
	address         *net.UDPAddr
	power           uint16
	label           string
	hardwareVersion stateVersion

	locationID string
	groupID    string

	sequence      uint8
	requestSocket *net.UDPConn
	responseMap   responseMap
	doneMap       doneMap
	responseInput packet.Chan
	subscriptions map[string]*common.Subscription
	quitChan      chan struct{}
	timeout       *time.Duration
	retryInterval *time.Duration
	limiter       *time.Timer
	seen          time.Time
	reliable      bool
	sync.RWMutex
}

type stateService struct {
	Service shared.Service `struc:"little"`
	Port    uint32         `struc:"little"`
}

type stateVersion struct {
	Vendor  uint32 `struc:"little"`
	Product uint32 `struc:"little"`
	Version uint32 `struc:"little"`
}

type stateLabel struct {
	Label [32]byte `struc:"little"`
}

type statePower struct {
	Level uint16 `struc:"little"`
}

type payloadPower struct {
	Level uint16 `struc:"little"`
}

type payloadLabel struct {
	Label [32]byte `struc:"little"`
}

func (d *Device) init(addr *net.UDPAddr, requestSocket *net.UDPConn, timeout *time.Duration, retryInterval *time.Duration, reliable bool) {
	d.address = addr
	d.requestSocket = requestSocket
	d.timeout = timeout
	d.retryInterval = retryInterval
	d.reliable = reliable
	d.limiter = time.NewTimer(shared.RateLimit)
	d.responseMap = make(responseMap)
	d.doneMap = make(doneMap)
	d.responseInput = make(packet.Chan, 32)
	d.subscriptions = make(map[string]*common.Subscription)
	d.quitChan = make(chan struct{})
}

func (d *Device) ID() uint64 {
	return d.id
}

func (d *Device) Discover() error {
	pkt := packet.New(d.address, d.requestSocket)
	pkt.SetType(GetService)
	_, err := d.Send(pkt, false, false)
	if err != nil {
		return err
	}

	return nil
}

// NewSubscription returns a new *common.Subscription for receiving events from
// this device.
func (d *Device) NewSubscription() (*common.Subscription, error) {
	sub := common.NewSubscription(d)
	d.Lock()
	d.subscriptions[sub.ID()] = sub
	d.Unlock()
	return sub, nil
}

// CloseSubscription is a callback for handling the closing of subscriptions.
func (d *Device) CloseSubscription(sub *common.Subscription) error {
	d.RLock()
	_, ok := d.subscriptions[sub.ID()]
	d.RUnlock()
	if !ok {
		return common.ErrNotFound
	}
	d.Lock()
	delete(d.subscriptions, sub.ID())
	d.Unlock()

	return nil
}

func (d *Device) SetStateLabel(pkt *packet.Packet) error {
	l := stateLabel{}
	if err := pkt.DecodePayload(&l); err != nil {
		return err
	}
	common.Log.Debugf("Got label (%v): %+v\n", d.id, l.Label)
	newLabel := stripNull(string(l.Label[:]))
	if newLabel != d.label {
		d.label = newLabel
		if err := d.publish(common.EventUpdateLabel{Label: d.label}); err != nil {
			return err
		}
	}

	return nil
}

func (d *Device) SetStateLocation(pkt *packet.Packet) error {
	l := new(Location)
	if err := l.Parse(pkt); err != nil {
		return err
	}
	common.Log.Debugf("Got location (%v): %+v (%+v)\n", d.id, l.ID(), l.GetLabel())
	newLocation := l.ID()
	if newLocation != d.locationID {
		d.locationID = newLocation
		// TODO: Work out what to notify on without causing protocol version
		// dependency
	}

	return nil
}

func (d *Device) SetStateGroup(pkt *packet.Packet) error {
	l := new(Group)
	if err := l.Parse(pkt); err != nil {
		return err
	}
	common.Log.Debugf("Got group (%v): %+v (%+v)\n", d.id, l.ID(), l.GetLabel())
	newGroup := l.ID()
	if newGroup != d.groupID {
		d.groupID = newGroup
		// TODO: Work out what to notify on without causing protocol version
		// dependency
	}

	return nil
}

func (d *Device) GetLabel() (string, error) {
	if len(d.label) != 0 {
		return d.label, nil
	}

	pkt := packet.New(d.address, d.requestSocket)
	pkt.SetType(GetLabel)
	req, err := d.Send(pkt, d.reliable, true)
	if err != nil {
		return ``, err
	}

	common.Log.Debugf("Waiting for label (%v)\n", d.id)
	pktResponse := <-req
	if pktResponse.Error != nil {
		return ``, err
	}

	err = d.SetStateLabel(&pktResponse.Result)
	if err != nil {
		return ``, err
	}

	return d.label, nil
}

func (d *Device) SetLabel(label string) error {
	if d.label == label {
		return nil
	}

	p := new(payloadLabel)
	copy(p.Label[:], label)

	pkt := packet.New(d.address, d.requestSocket)
	pkt.SetType(SetLabel)
	if err := pkt.SetPayload(p); err != nil {
		return err
	}

	common.Log.Debugf("Setting label on %v: %v\n", d.id, label)
	req, err := d.Send(pkt, d.reliable, false)
	if err != nil {
		return err
	}
	if d.reliable {
		// Wait for ack
		<-req
		common.Log.Debugf("Setting label on %v acknowledged\n", d.id)
	}

	d.label = label
	if err := d.publish(common.EventUpdateLabel{Label: d.label}); err != nil {
		return err
	}
	return nil
}

func (d *Device) SetStatePower(pkt *packet.Packet) error {
	p := statePower{}
	if err := pkt.DecodePayload(&p); err != nil {
		return err
	}
	common.Log.Debugf("Got power (%v): %+v\n", d.id, d.power)

	if d.power != p.Level {
		d.Lock()
		d.power = p.Level
		d.Unlock()
		if err := d.publish(common.EventUpdatePower{Power: d.power > 0}); err != nil {
			return err
		}
	}

	return nil
}

func (d *Device) GetPower() (bool, error) {
	pkt := packet.New(d.address, d.requestSocket)
	pkt.SetType(GetPower)
	req, err := d.Send(pkt, d.reliable, true)
	if err != nil {
		return false, err
	}

	common.Log.Debugf("Waiting for power (%v)\n", d.id)
	pktResponse := <-req
	if pktResponse.Error != nil {
		return false, err
	}

	err = d.SetStatePower(&pktResponse.Result)
	if err != nil {
		return false, err
	}

	return d.power > 0, nil
}

func (d *Device) CachedPower() bool {
	d.RLock()
	state := d.power
	d.RUnlock()
	return state != 0
}

func (d *Device) SetPower(state bool) error {
	if state && d.power > 0 {
		return nil
	}

	p := new(payloadPower)
	if state {
		p.Level = math.MaxUint16
	}

	pkt := packet.New(d.address, d.requestSocket)
	pkt.SetType(SetPower)
	if err := pkt.SetPayload(p); err != nil {
		return err
	}

	common.Log.Debugf("Setting power state on %v: %v\n", d.id, state)
	req, err := d.Send(pkt, d.reliable, false)
	if err != nil {
		return err
	}
	if d.reliable {
		// Wait for ack
		<-req
		common.Log.Debugf("Setting power state on %v acknowledged\n", d.id)
	}

	d.power = p.Level
	if err := d.publish(common.EventUpdatePower{Power: d.power > 0}); err != nil {
		return err
	}
	return nil
}

func (d *Device) GetLocation() (ret string, err error) {
	if d.locationID != ret {
		return d.locationID, nil
	}

	pkt := packet.New(d.address, d.requestSocket)
	pkt.SetType(GetLocation)
	req, err := d.Send(pkt, d.reliable, true)
	if err != nil {
		return ret, err
	}

	common.Log.Debugf("Waiting for location (%v)\n", d.id)
	pktResponse := <-req
	if pktResponse.Error != nil {
		return ret, err
	}

	err = d.SetStateLocation(&pktResponse.Result)
	if err != nil {
		return ret, err
	}

	return d.locationID, nil
}

func (d *Device) GetGroup() (ret string, err error) {
	if d.groupID != ret {
		return d.groupID, nil
	}

	pkt := packet.New(d.address, d.requestSocket)
	pkt.SetType(GetGroup)
	req, err := d.Send(pkt, d.reliable, true)
	if err != nil {
		return ret, err
	}

	common.Log.Debugf("Waiting for group (%v)\n", d.id)
	pktResponse := <-req
	if pktResponse.Error != nil {
		return ret, err
	}

	err = d.SetStateGroup(&pktResponse.Result)
	if err != nil {
		return ret, err
	}

	return d.groupID, nil
}

func (d *Device) GetHardwareVendor() (uint32, error) {
	if d.hardwareVersion.Product != 0 {
		return d.hardwareVersion.Vendor, nil
	}
	_, err := d.GetHardwareVersion()
	if err != nil {
		return 0, err
	}

	return d.hardwareVersion.Vendor, nil
}

func (d *Device) GetHardwareProduct() (uint32, error) {
	if d.hardwareVersion.Product != 0 {
		return d.hardwareVersion.Product, nil
	}
	_, err := d.GetHardwareVersion()
	if err != nil {
		return 0, err
	}

	return d.hardwareVersion.Product, nil
}

func (d *Device) GetHardwareVersion() (uint32, error) {
	if d.hardwareVersion.Product != 0 {
		return d.hardwareVersion.Version, nil
	}

	pkt := packet.New(d.address, d.requestSocket)
	pkt.SetType(GetVersion)
	req, err := d.Send(pkt, d.reliable, true)
	if err != nil {
		return 0, err
	}

	common.Log.Debugf("Waiting for hardware version (%v)\n", d.id)
	pktResponse := <-req
	if pktResponse.Error != nil {
		return 0, err
	}

	if err = pktResponse.Result.DecodePayload(&d.hardwareVersion); err != nil {
		return 0, err
	}
	common.Log.Debugf("Got hardware version (%v): %+v\n", d.id, d.hardwareVersion)

	return d.hardwareVersion.Version, nil
}

func (d *Device) Handle(pkt *packet.Packet) {
	d.responseInput <- packet.Response{Result: *pkt}
}

func (d *Device) GetAddress() *net.UDPAddr {
	return d.address
}

func (d *Device) ResetLimiter() {
	d.limiter.Reset(shared.RateLimit)
}

func (d *Device) resetLimiter(broadcast bool) {
	if broadcast {
		if err := d.publish(shared.EventRequestSent{}); err != nil {
			common.Log.Warnf("Failed publishing EventRequestSent on dev %+v: %+v\n", d.id, err)
		}
	} else {
		if err := d.publish(shared.EventBroadcastSent{}); err != nil {
			common.Log.Warnf("Failed publishing EventBroadcastSent on dev %+v: %+v\n", d.id, err)
		}
	}
	d.ResetLimiter()
}

func (d *Device) Send(pkt *packet.Packet, ackRequired, responseRequired bool) (packet.Chan, error) {
	proxyChan := make(packet.Chan)

	// Rate limiter
	<-d.limiter.C

	// Broadcast vs direct
	broadcast := d.id == 0
	if broadcast {
		// Broadcast can't be reliable
		ackRequired = false
		pkt.SetTagged(true)
	} else {
		pkt.SetTarget(d.id)
		if ackRequired {
			pkt.SetAckRequired(true)
		}
		if responseRequired {
			pkt.SetResRequired(true)
		}
		if ackRequired || responseRequired {
			inputChan := make(packet.Chan)
			doneChan := make(chan struct{})

			d.Lock()
			d.sequence++
			if d.sequence == 0 {
				d.sequence++
			}
			seq := d.sequence
			d.responseMap[seq] = inputChan
			d.doneMap[seq] = doneChan
			pkt.SetSequence(seq)
			d.Unlock()

			go func() {
				defer func() {
					close(doneChan)
				}()

				var (
					ok          bool
					timeout     <-chan time.Time
					pktResponse = packet.Response{}
					ticker      = time.NewTicker(*d.retryInterval)
				)

				if d.timeout == nil || *d.timeout == 0 {
					timeout = make(<-chan time.Time)
				} else {
					timeout = time.After(*d.timeout)
				}

				for {
					select {
					case pktResponse, ok = <-inputChan:
						if !ok {
							close(proxyChan)
							return
						}
						if pktResponse.Result.GetType() == Acknowledgement {
							common.Log.Debugf("Got ACK for seq %d on device %d, cancelling retries\n", seq, d.ID())
							ticker.Stop()
							if responseRequired {
								continue
							}
						}
						proxyChan <- pktResponse
						return
					case <-ticker.C:
						common.Log.Debugf("Retrying send after %d milliseconds: %+v\n", *d.retryInterval/time.Millisecond, *pkt)
						if err := pkt.Write(); err != nil {
							pktResponse.Error = err
							proxyChan <- pktResponse
							return
						}
					case <-timeout:
						pktResponse.Error = common.ErrTimeout
						proxyChan <- pktResponse
						return
					}
				}
			}()
		}
	}

	err := pkt.Write()
	d.resetLimiter(broadcast)

	return proxyChan, err
}

func (d *Device) Seen() time.Time {
	d.RLock()
	seen := d.seen
	d.RUnlock()
	return seen
}

func (d *Device) SetSeen(seen time.Time) {
	d.Lock()
	d.seen = seen
	d.Unlock()
}

// Close cleans up Device resources
func (d *Device) Close() error {
	for _, sub := range d.subscriptions {
		if err := sub.Close(); err != nil {
			return err
		}
	}

	d.Lock()
	defer d.Unlock()

	select {
	case <-d.quitChan:
		common.Log.Warnf(`device already closed`)
		return common.ErrClosed
	default:
		close(d.quitChan)
	}

	return nil
}

func (d *Device) handler() {
	var (
		pktResponse packet.Response
		ok          bool
		ch          packet.Chan
		done        chan struct{}
	)
	for {

		select {
		case <-d.quitChan:
			d.Lock()
			for seq, ch := range d.responseMap {
				ch <- packet.Response{Error: common.ErrClosed}
				close(ch)
				delete(d.responseMap, seq)
				delete(d.doneMap, seq)
			}
			d.Unlock()
			return
		case pktResponse = <-d.responseInput:
			common.Log.Debugf("Handling packet on device %v: %+v\n", d.id, pktResponse)
			seq := pktResponse.Result.GetSequence()
			d.RLock()
			ch, ok = d.responseMap[seq]
			if ok {
				done, ok = d.doneMap[seq]
			}
			d.RUnlock()
			if !ok {
				common.Log.Warnf("Couldn't find requestor for seq %v on device %v: %+v\n", seq, d.id, pktResponse)
				continue
			}
			common.Log.Debugf("Returning packet to caller on device %v: %+v\n", d.id, pktResponse)
			select {
			case ch <- pktResponse:
			case <-done:
				d.Lock()
				close(ch)
				delete(d.responseMap, seq)
				delete(d.doneMap, seq)
				d.Unlock()
			}
		}
	}
}

// Pushes an event to subscribers
func (d *Device) publish(event interface{}) error {
	d.RLock()
	subs := make(map[string]*common.Subscription, len(d.subscriptions))
	for k, sub := range d.subscriptions {
		subs[k] = sub
	}
	d.RUnlock()

	for _, sub := range subs {
		if err := sub.Write(event); err != nil {
			return err
		}
	}

	return nil
}

func New(addr *net.UDPAddr, requestSocket *net.UDPConn, timeout *time.Duration, retryInterval *time.Duration, reliable bool, pkt *packet.Packet) (*Device, error) {
	d := new(Device)
	d.init(addr, requestSocket, timeout, retryInterval, reliable)

	if pkt != nil {
		d.id = pkt.Target
		service := new(stateService)

		if err := pkt.DecodePayload(service); err != nil {
			return nil, err
		}

		d.address.Port = int(service.Port)
	}
	go d.handler()

	return d, nil
}
