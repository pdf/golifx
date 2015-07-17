package golifx_test

import (
	"github.com/pdf/golifx"
	"github.com/pdf/golifx/protocol"
)

// Instantiating a new client with protocol V2
func ExampleNewClient_v2() {
	client, err := golifx.NewClient(&protocol.V2{})
	if err != nil {
		panic(err)
	}
	client.GetLightByLabel(`lightLabel`)
}

// When using protocol V2, it's possible to choose an alternative client port.
// This is not recommended unless you need to use multiple client instances at
// the same time.
func ExampleNewClient_v2port() {
	client, err := golifx.NewClient(&protocol.V2{Port: 56701})
	if err != nil {
		panic(err)
	}
	client.GetLightByLabel(`lightLabel`)
}
