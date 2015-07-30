// Package device implements a LIFX LAN protocol version 2 device.
//
// This package is not designed to be accessed by end users, all interaction
// should occur via the Client in the golifx package.
package device

import (
	"math"
	"net"
	"strings"
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
	EchoRequest       shared.Message = 58
	EchoResponse      shared.Message = 59

	VendorLifx          = 1
	ProductLifxOriginal = 1
)

type responseMap map[uint8]packet.Chan

type GenericDevice interface {
	common.Device
	Handle(*packet.Packet)
	Close() error
	Seen() time.Time
	SetSeen(time.Time)
	SetStatePower(*packet.Packet) error
	SetStateLabel(*packet.Packet) error
}

type Device struct {
	id              uint64
	address         *net.UDPAddr
	power           uint16
	label           string
	hardwareVersion stateVersion

	sequence      uint8
	requestSocket *net.UDPConn
	responseMap   responseMap
	responseInput packet.Chan
	subscriptions map[string]*common.Subscription
	quitChan      chan bool
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
	common.Log.Debugf("Got label (%v): %+v\n", d.id, d.label)
	newLabel := stripNull(string(l.Label[:]))
	if newLabel != d.label {
		d.label = newLabel
		d.publish(common.EventUpdateLabel{Label: d.label})
	}

	return nil
}

func (d *Device) GetLabel() (string, error) {
	if len(d.label) != 0 {
		return d.label, nil
	}

	pkt := packet.New(d.address, d.requestSocket)
	pkt.SetType(GetLabel)
	req, err := d.Send(pkt, false, true)
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
	pkt.SetPayload(p)

	common.Log.Debugf("Setting label on %v: %v\n", d.id, label)
	if _, err := d.Send(pkt, false, false); err != nil {
		return err
	}

	d.label = label
	d.publish(common.EventUpdateLabel{Label: d.label})
	return nil
}

func (d *Device) SetStatePower(pkt *packet.Packet) error {
	p := statePower{}
	if err := pkt.DecodePayload(&p); err != nil {
		return err
	}
	common.Log.Debugf("Got power (%v): %+v\n", d.id, d.power)

	if d.power != p.Level {
		d.power = p.Level
		d.publish(common.EventUpdatePower{Power: d.power > 0})
	}

	return nil
}

func (d *Device) GetPower() (bool, error) {
	if d.power != 0 {
		return true, nil
	}
	pkt := packet.New(d.address, d.requestSocket)
	pkt.SetType(GetPower)
	req, err := d.Send(pkt, false, true)
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
	pkt.SetPayload(p)

	common.Log.Debugf("Setting power state on %v: %v\n", d.id, state)
	if _, err := d.Send(pkt, false, false); err != nil {
		return err
	}

	d.power = p.Level
	d.publish(common.EventUpdatePower{Power: d.power > 0})
	return nil
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
	req, err := d.Send(pkt, false, true)
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

func (d *Device) Send(pkt *packet.Packet, ackRequired, responseRequired bool) (packet.Chan, error) {
	proxyChan := make(packet.Chan)

	if d.reliable {
		ackRequired = true
	}

	// Rate limiter
	<-d.limiter.C

	// Broadcast vs direct
	if d.id == 0 {
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

			d.Lock()
			d.sequence++
			if d.sequence == 0 {
				d.sequence++
			}
			d.responseMap[d.sequence] = inputChan
			pkt.SetSequence(d.sequence)
			d.Unlock()

			go func() {
				var timeout <-chan time.Time
				pktResponse := packet.Response{}
				ticker := time.NewTicker(*d.retryInterval)
				if d.timeout == nil || *d.timeout == 0 {
					timeout = make(<-chan time.Time)
				} else {
					timeout = time.After(*d.timeout)
				}
				for {
					select {
					case <-d.quitChan:
						// Re-populate the quitChan for other routines
						d.quitChan <- true
						return
					case pktResponse = <-inputChan:
						if pktResponse.Result.GetType() == Acknowledgement {
							common.Log.Debugf("Got ACK for seq %d on device %d, cancelling retries\n", pkt.GetSequence(), d.ID())
							ticker.Stop()
							if responseRequired {
								continue
							}
							return
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
	d.ResetLimiter()

	return proxyChan, err
}

func (d *Device) Seen() time.Time {
	return d.seen
}

func (d *Device) SetSeen(seen time.Time) {
	d.seen = seen
}

func (d *Device) Close() error {
	common.Log.Debugf("Closing device %v", d.id)
	d.Lock()
	d.quitChan <- true
	d.Unlock()

	return nil
}

func (d *Device) handler() {
	var pktResponse packet.Response
	for {
		select {
		case <-d.quitChan:
			// Re-populate the quitChan for other routines
			d.quitChan <- true
			return
		case pktResponse = <-d.responseInput:
			common.Log.Debugf("Handling packet on device %v: %+v\n", d.id, pktResponse)
			seq := pktResponse.Result.GetSequence()
			d.RLock()
			ch, ok := d.responseMap[seq]
			d.RUnlock()
			if !ok {
				common.Log.Warnf("Couldn't find requestor for seq %v on device %v: %+v\n", seq, d.id, pktResponse)
				continue
			}
			common.Log.Debugf("Returning packet to caller on device %v: %+v\n", d.id, pktResponse)
			ch <- pktResponse
			if pktResponse.Result.GetType() != Acknowledgement {
				d.Lock()
				close(ch)
				delete(d.responseMap, seq)
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
	d := &Device{
		address:       addr,
		requestSocket: requestSocket,
		responseMap:   make(responseMap),
		responseInput: make(packet.Chan, 32),
		subscriptions: make(map[string]*common.Subscription),
		quitChan:      make(chan bool),
		timeout:       timeout,
		retryInterval: retryInterval,
		reliable:      reliable,
		limiter:       time.NewTimer(shared.RateLimit),
	}

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

func stripNull(s string) string {
	return strings.Replace(s, string(0), ``, -1)
}
