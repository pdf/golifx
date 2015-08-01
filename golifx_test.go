package golifx_test

import (
	"github.com/pdf/golifx"
	"github.com/pdf/golifx/common"
	"github.com/pdf/golifx/protocol"
)

// Instantiating a new client with protocol V2, with default options
func ExampleNewClient_v2() common.Light {
	client, err := golifx.NewClient(&protocol.V2{})
	if err != nil {
		panic(err)
	}
	light, err := client.GetLightByLabel(`lightLabel`)
	if err != nil {
		panic(err)
	}
	return light
}

// When using protocol V2, it's possible to choose an alternative client port.
// This is not recommended unless you need to use multiple client instances at
// the same time.
func ExampleNewClient_v2port() common.Light {
	client, err := golifx.NewClient(&protocol.V2{Port: 56701})
	if err != nil {
		panic(err)
	}
	light, err := client.GetLightByLabel(`lightLabel`)
	if err != nil {
		panic(err)
	}
	return light
}

// When using protocol V2, it's possible to enable reliable operations.  This is
// highly recommended, otherwise sends operate in fire-and-forget mode, meaning
// we don't know if they actually arrived at the target or not.  Whilst this is
// faster and generates less network traffic, in my experience LIFX devices
// aren't the most reliable at accepting the packets they're sent, so sometimes
// we need to retry.
func ExampleNewClient_v2reliable() common.Light {
	client, err := golifx.NewClient(&protocol.V2{Reliable: true})
	if err != nil {
		panic(err)
	}
	light, err := client.GetLightByLabel(`lightLabel`)
	if err != nil {
		panic(err)
	}
	return light
}
