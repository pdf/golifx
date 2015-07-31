package golifx

import (
	"sync"
	"time"

	"github.com/pdf/golifx/common"
)

// Client provides a simple interface for interacting with LIFX devices.  Client
// can not be instantiated manually or it will not function - always use
// NewClient() to obtain a Client instance.
type Client struct {
	discoveryInterval     time.Duration
	quitChan              chan bool
	protocol              common.Protocol
	timeout               time.Duration
	retryInterval         time.Duration
	internalRetryInterval time.Duration
	devices               map[uint64]common.Device
	subscriptions         map[string]*common.Subscription
	sync.RWMutex
}

// addDevice adds dev to the client's known devices, and returns dev.  Returns
// common.ErrDuplicate if the device is already known.
func (c *Client) addDevice(dev common.Device) error {
	id := dev.ID()
	c.RLock()
	_, ok := c.devices[id]
	c.RUnlock()
	if ok {
		return common.ErrDuplicate
	}

	c.Lock()
	c.devices[id] = dev
	c.Unlock()

	err := c.publish(common.EventNewDevice{Device: dev})
	if err != nil {
		common.Log.Warnf("Failed publishing event: %v\n", err)
	}

	return nil
}

// removeDeviceByID looks up a device by it's id and removes it from the
// client's list of known devices, or returns common.ErrNotFound if the device
// is not known at this time.
func (c *Client) removeDeviceByID(id uint64) error {
	c.RLock()
	dev, ok := c.devices[id]
	c.RUnlock()
	if !ok {
		return common.ErrNotFound
	}

	c.Lock()
	delete(c.devices, id)
	c.Unlock()

	err := c.publish(common.EventExpiredDevice{Device: dev})
	if err != nil {
		common.Log.Warnf("Failed publishing event: %v\n", err)
	}

	return nil
}

// GetDevices returns a slice of all devices known to the client, or
// common.ErrNotFound if no devices are currently known.
func (c *Client) GetDevices() ([]common.Device, error) {
	c.RLock()
	devices := make([]common.Device, len(c.devices))
	c.RUnlock()
	if len(devices) == 0 {
		return devices, common.ErrNotFound
	}
	i := 0
	c.RLock()
	for _, dev := range c.devices {
		devices[i] = dev
		i++
	}
	c.RUnlock()
	return devices, nil
}

// GetDeviceByID looks up a device by it's id and returns a common.Device.
// May return a common.ErrNotFound error if the lookup times out without finding
// the device.
func (c *Client) GetDeviceByID(id uint64) (common.Device, error) {
	c.RLock()
	dev, ok := c.devices[id]
	c.RUnlock()
	if ok {
		return dev, nil
	}

	var timeout <-chan time.Time
	if c.timeout > 0 {
		timeout = time.After(c.timeout)
	} else {
		timeout = make(<-chan time.Time)
	}

	sub, err := c.protocol.NewSubscription()
	if err != nil {
		return nil, err
	}
	events := sub.Events()

	for {
		select {
		case event := <-events:
			switch event := event.(type) {
			case common.EventNewDevice:
				if event.Device.ID() == id {
					return event.Device, nil
				}
			}
		case <-timeout:
			return nil, common.ErrNotFound
		}
	}
}

// GetDeviceByLabel looks up a device by it's label and returns a common.Device.
// May return a common.ErrNotFound error if the lookup times out without finding
// the device.
func (c *Client) GetDeviceByLabel(label string) (common.Device, error) {
	devices, _ := c.GetDevices()
	for _, dev := range devices {
		res, err := dev.GetLabel()
		if err == nil && res == label {
			return dev, nil
		}
	}

	var timeout <-chan time.Time
	if c.timeout > 0 {
		timeout = time.After(c.timeout)
	} else {
		timeout = make(<-chan time.Time)
	}

	sub, err := c.protocol.NewSubscription()
	if err != nil {
		return nil, err
	}
	events := sub.Events()

	for {
		select {
		case event := <-events:
			switch event := event.(type) {
			case common.EventNewDevice:
				l, err := event.Device.GetLabel()
				if err != nil {
					return nil, err
				}
				if l == label {
					return event.Device, nil
				}
			}
		case <-timeout:
			return nil, common.ErrNotFound
		}
	}
}

// GetLights returns a slice of all lights known to the client, or
// common.ErrNotFound if no lights are currently known.
func (c *Client) GetLights() (lights []common.Light, err error) {
	devices, err := c.GetDevices()
	if err != nil {
		return lights, err
	}

	for _, dev := range devices {
		if light, ok := dev.(common.Light); ok {
			lights = append(lights, light)
		}
	}

	if len(lights) == 0 {
		return lights, common.ErrNotFound
	}

	return lights, nil
}

// GetLightByID looks up a light by it's id and returns a common.Light.
// May return a common.ErrNotFound error if the lookup times out without finding
// the light, or common.ErrDeviceInvalidType if the device exists but is not a
// light.
func (c *Client) GetLightByID(id uint64) (light common.Light, err error) {
	dev, err := c.GetDeviceByID(id)
	if err != nil {
		return nil, err
	}

	light, ok := dev.(common.Light)
	if !ok {
		return nil, common.ErrDeviceInvalidType
	}

	return light, nil
}

// GetLightByLabel looks up a light by it's label and returns a common.Light.
// May return a common.ErrNotFound error if the lookup times out without finding
// the light, or common.ErrDeviceInvalidType if the device exists but is not a
// light.
func (c *Client) GetLightByLabel(label string) (common.Light, error) {
	dev, err := c.GetDeviceByLabel(label)
	if err != nil {
		return nil, err
	}

	light, ok := dev.(common.Light)
	if !ok {
		return nil, common.ErrDeviceInvalidType
	}

	return light, nil
}

// SetPower broadcasts a request to change the power state of all devices on
// the network.  A state of true requests power on, and a state of false
// requests power off.
func (c *Client) SetPower(state bool) error {
	return c.protocol.SetPower(state)
}

// SetPowerDuration broadcasts a request to change the power state of all
// devices on the network, transitioning over the specified duration.  A state
// of true requests power on, and a state of false requests power off.  Not all
// device types support transitioning, so if you wish to change the state of all
// device types, you should use SetPower instead.
func (c *Client) SetPowerDuration(state bool, duration time.Duration) error {
	return c.protocol.SetPowerDuration(state, duration)
}

// SetColor broadcasts a request to change the color of all devices on the
// network.
func (c *Client) SetColor(color common.Color, duration time.Duration) error {
	return c.protocol.SetColor(color, duration)
}

// SetDiscoveryInterval causes the client to discover devices and state every
// interval.  You should set this to a non-zero value for any long-running
// process, otherwise devices will only be discovered once.
func (c *Client) SetDiscoveryInterval(interval time.Duration) error {
	c.Lock()
	if c.discoveryInterval != 0 {
		for i := 0; i < cap(c.quitChan); i++ {
			c.quitChan <- true
		}
	}
	c.discoveryInterval = interval
	c.Unlock()
	common.Log.Infof("Starting discovery with interval %v", interval)
	return c.discover()
}

// SetTimeout sets the time that client operations wait for results before
// returning an error.  The special value of 0 may be set to disable timeouts,
// and all operations will wait indefinitely, but this is not recommended.
func (c *Client) SetTimeout(timeout time.Duration) {
	c.timeout = timeout
}

// GetTimeout returns the currently configured timeout period for operations on
// this client
func (c *Client) GetTimeout() *time.Duration {
	return &c.timeout
}

// SetRetryInterval sets the retry interval for operations on this client.  If
// a timeout has been set, and the retry interval exceeds the timeout, the retry
// interval will be set to half the timeout
func (c *Client) SetRetryInterval(retryInterval time.Duration) {
	if c.timeout > 0 && retryInterval >= c.timeout {
		retryInterval = c.timeout / 2
	}
	c.retryInterval = retryInterval
}

// GetRetryInterval returns the currently configured retry interval for
// operations on this client
func (c *Client) GetRetryInterval() *time.Duration {
	return &c.retryInterval
}

// NewSubscription returns a new *common.Subscription for receiving events from
// this client.
func (c *Client) NewSubscription() (*common.Subscription, error) {
	sub := common.NewSubscription(c)
	c.Lock()
	c.subscriptions[sub.ID()] = sub
	c.Unlock()
	return sub, nil
}

// CloseSubscription is a callback for handling the closing of subscriptions.
func (c *Client) CloseSubscription(sub *common.Subscription) error {
	c.RLock()
	_, ok := c.subscriptions[sub.ID()]
	c.RUnlock()
	if !ok {
		return common.ErrNotFound
	}
	c.Lock()
	delete(c.subscriptions, sub.ID())
	c.Unlock()

	return nil
}

// Close signals the termination of this client, and cleans up resources
func (c *Client) Close() error {
	for _, sub := range c.subscriptions {
		sub.Close()
	}
	c.Lock()
	defer c.Unlock()
	c.quitChan <- true
	return c.protocol.Close()
}

// Pushes an event to subscribers
func (c *Client) publish(event interface{}) error {
	c.RLock()
	subs := make(map[string]*common.Subscription, len(c.subscriptions))
	for k, sub := range c.subscriptions {
		subs[k] = sub
	}
	c.RUnlock()

	for _, sub := range subs {
		if err := sub.Write(event); err != nil {
			return err
		}
	}

	return nil
}

func (c *Client) subscribe() error {
	sub, err := c.protocol.NewSubscription()
	if err != nil {
		return err
	}
	events := sub.Events()
	go func() {
		for {
			select {
			case <-c.quitChan:
				sub.Close()
				return
			case event := <-events:
				switch event := event.(type) {
				case common.EventNewDevice:
					if err := c.addDevice(event.Device); err != nil {
						common.Log.Debugf("Could not add device to client: %v\n", err)
					}
				case common.EventExpiredDevice:
					if err := c.removeDeviceByID(event.Device.ID()); err != nil {
						common.Log.Debugf("Could not remove device from client: %v\n", err)
					}
				default:
					common.Log.Debugf("Unhandled event on client: %+v\n", event)
					continue
				}
			}
		}
	}()

	return nil
}

func (c *Client) discover() error {
	if err := c.subscribe(); err != nil {
		return err
	}
	if c.discoveryInterval == 0 {
		common.Log.Debugf("Discovery interval is zero, discovery will only be performed once")
		return c.protocol.Discover()
	}

	go func() {
		tick := time.Tick(c.discoveryInterval)
		for {
			select {
			case <-c.quitChan:
				common.Log.Debugf("Quitting discovery loop")
				return
			case <-tick:
				common.Log.Debugf("Performing discovery")
				_ = c.protocol.Discover()
			}
		}
	}()

	return nil
}
