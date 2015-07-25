[![Build Status](https://drone.io/github.com/pdf/golifx/status.png)](https://drone.io/github.com/pdf/golifx/latest) [![GoDoc](https://godoc.org/github.com/pdf/golifx?status.svg)](http://godoc.org/github.com/pdf/golifx) ![License-MIT](http://img.shields.io/badge/license-MIT-red.svg)

__Note:__ This library is at a moderately early stage - everything should work,
but the V2 protocol implementation needs documentation and testing.

If you have Go installed, you can install the `lifx` CLI application like so:

```shell
go get github.com/pdf/golifx/cmd/lifx
```

The `lifx` command will be available at `${GOPATH}/bin/lifx`

# golifx
--
    import "github.com/pdf/golifx"

Package golifx provides a simple Go interface to the LIFX LAN protocol.

Based on the protocol documentation available at: http://lan.developer.lifx.com/

Also included in cmd/lifx is a small CLI utility that allows interacting with
your LIFX devices on the LAN.

## Usage

```go
const (
	// VERSION of this library
	VERSION = `0.0.1`
)
```

#### func  SetLogger

```go
func SetLogger(logger common.Logger)
```
SetLogger allows assigning a custom levelled logger that conforms to the
common.Logger interface. To capture logs generated during client creation, this
should be called before creating a Client. Defaults to common.StubLogger, which
does no logging at all.

#### type Client

```go
type Client struct {
	sync.RWMutex
}
```

Client provides a simple interface for interacting with LIFX devices. Client can
not be instantiated manually or it will not function - always use NewClient() to
obtain a Client instance.

#### func  NewClient

```go
func NewClient(p protocol.Protocol) (*Client, error)
```
NewClient returns a pointer to a new Client and any error that occurred
initializing the client, using the protocol p. It also kicks off a discovery
run.

#### func (*Client) AddDevice

```go
func (c *Client) AddDevice(dev common.Device) error
```
AddDevice is for use by protocols only. Adds dev to the client's known devices,
and returns dev. Returns common.ErrDuplicate if the device is already known.

#### func (*Client) Close

```go
func (c *Client) Close() error
```
Close signals the termination of this client, and cleans up resources

#### func (*Client) GetDeviceByID

```go
func (c *Client) GetDeviceByID(id uint64) (common.Device, error)
```
GetDeviceByID looks up a device by it's id and returns a common.Device. May
return a common.ErrNotFound error if the lookup times out without finding the
device.

#### func (*Client) GetDeviceByLabel

```go
func (c *Client) GetDeviceByLabel(label string) (common.Device, error)
```
GetDeviceByLabel looks up a device by it's label and returns a common.Device.
May return a common.ErrNotFound error if the lookup times out without finding
the device.

#### func (*Client) GetDevices

```go
func (c *Client) GetDevices() ([]common.Device, error)
```
GetDevices returns a slice of all devices known to the client, or
common.ErrNotFound if no devices are currently known.

#### func (*Client) GetLightByID

```go
func (c *Client) GetLightByID(id uint64) (light common.Light, err error)
```
GetLightByID looks up a light by it's id and returns a common.Light. May return
a common.ErrNotFound error if the lookup times out without finding the light, or
common.ErrDeviceInvalidType if the device exists but is not a light.

#### func (*Client) GetLightByLabel

```go
func (c *Client) GetLightByLabel(label string) (light common.Light, err error)
```
GetLightByLabel looks up a light by it's label and returns a common.Light. May
return a common.ErrNotFound error if the lookup times out without finding the
light, or common.ErrDeviceInvalidType if the device exists but is not a light.

#### func (*Client) GetLights

```go
func (c *Client) GetLights() (lights []common.Light, err error)
```
GetLights returns a slice of all lights known to the client, or
common.ErrNotFound if no lights are currently known.

#### func (*Client) GetRetryInterval

```go
func (c *Client) GetRetryInterval() *time.Duration
```
GetRetryInterval returns the currently configured retry interval for operations
on this client

#### func (*Client) GetTimeout

```go
func (c *Client) GetTimeout() *time.Duration
```
GetTimeout returns the currently configured timeout period for operations on
this client

#### func (*Client) RemoveDeviceByID

```go
func (c *Client) RemoveDeviceByID(id uint64) error
```
RemoveDeviceByID is for use by protocols only. Looks up a device by it's id and
removes it from the client's list of known devices, or returns
common.ErrNotFound if the device is not known at this time.

#### func (*Client) SetColor

```go
func (c *Client) SetColor(color common.Color, duration time.Duration) error
```
SetColor broadcasts a request to change the color of all devices on the network.

#### func (*Client) SetDiscoveryInterval

```go
func (c *Client) SetDiscoveryInterval(interval time.Duration) error
```
SetDiscoveryInterval causes the client to discover devices and state every
interval. You should set this to a non-zero value for any long-running process,
otherwise devices will only be discovered once.

#### func (*Client) SetPower

```go
func (c *Client) SetPower(state bool) error
```
SetPower broadcasts a request to change the power state of all devices on the
network. A state of true requests power on, and a state of false requests power
off.

#### func (*Client) SetPowerDuration

```go
func (c *Client) SetPowerDuration(state bool, duration time.Duration) error
```
SetPowerDuration broadcasts a request to change the power state of all devices
on the network, transitioning over the specified duration. A state of true
requests power on, and a state of false requests power off. Not all device types
support transitioning, so if you wish to change the state of all device types,
you should use SetPower instead.

#### func (*Client) SetRetryInterval

```go
func (c *Client) SetRetryInterval(retryInterval time.Duration)
```
SetRetryInterval sets the retry interval for operations on this client. If a
timeout has been set, and the retry interval exceeds the timeout, the retry
interval will be set to half the timeout

#### func (*Client) SetTimeout

```go
func (c *Client) SetTimeout(timeout time.Duration)
```
SetTimeout sets the time that client operations wait for results before
returning an error
