package device

import (
	"encoding/base64"
	"strings"
	"sync"
	"time"

	"github.com/pdf/golifx/common"
	"github.com/pdf/golifx/protocol/v2/packet"
)

type stateGroup struct {
	ID        [16]byte `struc:"little"`
	Label     [32]byte `struc:"little"`
	UpdatedAt uint64   `struc:"little"`
}

type Group struct {
	id            [16]byte
	idEncoded     string
	label         [32]byte
	updatedAt     uint64
	devices       map[uint64]GenericDevice
	subscriptions map[string]*common.Subscription
	quitChan      chan struct{}
	sync.RWMutex
}

func (g *Group) init() {
	g.devices = make(map[uint64]GenericDevice)
	g.subscriptions = make(map[string]*common.Subscription)
	g.quitChan = make(chan struct{})
}

func (g *Group) ID() string {
	g.RLock()
	id := g.idEncoded
	g.RUnlock()
	return id
}

func (g *Group) GetLabel() string {
	return stripNull(string(g.label[:]))
}

func (g *Group) Devices() (devices []common.Device) {
	if len(g.devices) == 0 {
		return devices
	}
	g.RLock()
	for _, dev := range g.devices {
		devices = append(devices, dev.(common.Device))
	}
	g.RUnlock()

	return devices
}

func (g *Group) Lights() []common.Light {
	devices := g.Devices()
	lights := make([]common.Light, 0)
	for _, dev := range devices {
		if light, ok := dev.(common.Light); ok {
			lights = append(lights, light)
		}
	}

	return lights
}

func (g *Group) AddDevice(dev GenericDevice) error {
	g.RLock()
	_, ok := g.devices[dev.ID()]
	g.RUnlock()
	if ok {
		return common.ErrDuplicate
	}

	g.Lock()
	g.devices[dev.ID()] = dev
	g.Unlock()
	if err := g.addDeviceSubscription(dev); err != nil {
		return err
	}

	if err := g.publish(common.EventNewDevice{Device: dev}); err != nil {
		return err
	}

	return nil
}

func (g *Group) addDeviceSubscription(dev GenericDevice) error {
	sub, err := dev.NewSubscription()
	if err != nil {
		return err
	}
	events := sub.Events()

	go func() {
		for {
			select {
			case <-g.quitChan:
				return
			case event := <-events:
				switch event.(type) {
				case common.EventUpdateColor:
					color := g.CachedColor()
					if err != nil {
						continue
					}
					err = g.publish(common.EventUpdateColor{Color: color})
					if err != nil {
						continue
					}
				case common.EventUpdatePower:
					state := g.CachedPower()
					if err != nil {
						continue
					}
					err = g.publish(common.EventUpdatePower{Power: state})
					if err != nil {
						continue
					}
				}
			}
		}
	}()

	return nil
}

func (g *Group) RemoveDevice(dev GenericDevice) error {
	g.RLock()
	_, ok := g.devices[dev.ID()]
	g.RUnlock()
	if !ok {
		return common.ErrNotFound
	}

	g.Lock()
	delete(g.devices, dev.ID())
	g.Unlock()

	if err := g.publish(common.EventExpiredDevice{Device: dev}); err != nil {
		return err
	}

	return nil
}

func (g *Group) GetPower() (bool, error) {
	return g.getPower(false)
}

func (g *Group) CachedPower() bool {
	p, _ := g.getPower(true)
	return p
}

func (g *Group) getPower(cached bool) (bool, error) {
	var state uint
	devices := g.Devices()

	if len(devices) == 0 {
		return false, nil
	}

	for _, dev := range devices {
		var (
			p   bool
			err error
		)
		if cached {
			p = dev.CachedPower()
		} else {
			p, err = dev.GetPower()
			if err != nil {
				return false, err
			}
		}
		if p {
			state += 1
		}
	}

	return state > 0, nil
}

// GetColor returns the average color for lights in the group, or error if any
// light returns an error.
//
// I doubt this is accurate as color theory, but it's good enough for this
// use-case.
func (g *Group) GetColor() (common.Color, error) {
	return g.getColor(false)
}

func (g *Group) CachedColor() common.Color {
	c, _ := g.getColor(true)
	return c
}

func (g *Group) getColor(cached bool) (color common.Color, err error) {
	lights := g.Lights()

	if len(lights) == 0 {
		return color, nil
	}

	colors := make([]common.Color, len(lights))

	for i, light := range lights {
		var (
			c   common.Color
			err error
		)
		if cached {
			c = light.CachedColor()
		} else {
			c, err = light.GetColor()
			if err != nil {
				return color, err
			}
		}
		colors[i] = c
	}

	color = common.AverageColor(colors...)

	return color, nil
}

func (g *Group) SetColor(color common.Color, duration time.Duration) error {
	var (
		wg       sync.WaitGroup
		err      error
		errMutex sync.Mutex
	)

	lights := g.Lights()

	if len(lights) == 0 {
		return nil
	}

	for _, light := range lights {
		wg.Add(1)
		go func(light common.Light) {
			e := light.SetColor(color, duration)
			errMutex.Lock()
			if err == nil && e != nil {
				err = e
			}
			errMutex.Unlock()
			wg.Done()
		}(light)
	}

	wg.Wait()
	return err
}

func (g *Group) SetPower(state bool) error {
	var (
		wg       sync.WaitGroup
		err      error
		errMutex sync.Mutex
	)

	devices := g.Devices()

	if len(devices) == 0 {
		return nil
	}

	for _, device := range devices {
		wg.Add(1)
		go func(device common.Device) {
			e := device.SetPower(state)
			errMutex.Lock()
			if err == nil && e != nil {
				err = e
			}
			errMutex.Unlock()
			wg.Done()
		}(device)
	}

	wg.Wait()
	return err
}

func (g *Group) SetPowerDuration(state bool, duration time.Duration) error {
	var (
		wg       sync.WaitGroup
		err      error
		errMutex sync.Mutex
	)

	lights := g.Lights()

	if len(lights) == 0 {
		return nil
	}

	for _, light := range lights {
		wg.Add(1)
		go func(light common.Light) {
			e := light.SetPowerDuration(state, duration)
			errMutex.Lock()
			if err == nil && e != nil {
				err = e
			}
			errMutex.Unlock()
			wg.Done()
		}(light)
	}

	wg.Wait()
	return err
}

func (g *Group) Parse(pkt *packet.Packet) error {
	var shouldUpdate, labelUpdate bool

	s := stateGroup{}
	if err := pkt.DecodePayload(&s); err != nil {
		return err
	}

	g.RLock()
	if s.UpdatedAt > g.updatedAt {
		shouldUpdate = true
	}
	g.RUnlock()

	if shouldUpdate {
		g.Lock()
		g.id = s.ID
		g.idEncoded = strings.Replace(
			base64.URLEncoding.EncodeToString(s.ID[:]),
			`=`, ``, -1,
		)
		g.updatedAt = s.UpdatedAt
		if g.label != s.Label {
			g.label = s.Label
			labelUpdate = true
		}
		g.Unlock()

		if labelUpdate {
			if err := g.publish(common.EventUpdateLabel{Label: g.GetLabel()}); err != nil {
				return err
			}
		}
	}

	return nil
}

// NewSubscription returns a new *common.Subscription for receiving events from
// this group.
func (g *Group) NewSubscription() (*common.Subscription, error) {
	sub := common.NewSubscription(g)
	g.Lock()
	g.subscriptions[sub.ID()] = sub
	g.Unlock()
	return sub, nil
}

// CloseSubscription is a callback for handling the closing of subscriptions.
func (g *Group) CloseSubscription(sub *common.Subscription) error {
	g.RLock()
	_, ok := g.subscriptions[sub.ID()]
	g.RUnlock()
	if !ok {
		return common.ErrNotFound
	}
	g.Lock()
	delete(g.subscriptions, sub.ID())
	g.Unlock()

	return nil
}

// Close cleans up Group resources
func (g *Group) Close() error {
	for _, sub := range g.subscriptions {
		if err := sub.Close(); err != nil {
			return err
		}
	}

	g.Lock()
	defer g.Unlock()

	select {
	case <-g.quitChan:
		common.Log.Warnf(`group already closed`)
		return common.ErrClosed
	default:
		close(g.quitChan)
	}

	return nil
}

// Pushes an event to subscribers
func (g *Group) publish(event interface{}) error {
	g.RLock()
	subs := make(map[string]*common.Subscription, len(g.subscriptions))
	for k, sub := range g.subscriptions {
		subs[k] = sub
	}
	g.RUnlock()

	for _, sub := range subs {
		if err := sub.Write(event); err != nil {
			return err
		}
	}

	return nil
}

func NewGroup(pkt *packet.Packet) (*Group, error) {
	g := new(Group)
	g.init()
	if err := g.Parse(pkt); err != nil {
		return g, err
	}

	return g, nil
}
