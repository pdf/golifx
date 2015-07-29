package mocks

import "github.com/pdf/golifx/common"
import "github.com/stretchr/testify/mock"

import "time"

type Protocol struct {
	SubscriptionTarget
	mock.Mock
}

func (_m *Protocol) SetClient(client common.Client) {
	_m.Called(client)
}
func (_m *Protocol) Discover() error {
	ret := _m.Called()

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}
func (_m *Protocol) Close() error {
	ret := _m.Called()

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}
func (_m *Protocol) SetPower(state bool) error {
	ret := _m.Called(state)

	var r0 error
	if rf, ok := ret.Get(0).(func(bool) error); ok {
		r0 = rf(state)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}
func (_m *Protocol) SetPowerDuration(state bool, duration time.Duration) error {
	ret := _m.Called(state, duration)

	var r0 error
	if rf, ok := ret.Get(0).(func(bool, time.Duration) error); ok {
		r0 = rf(state, duration)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}
func (_m *Protocol) SetColor(color common.Color, duration time.Duration) error {
	ret := _m.Called(color, duration)

	var r0 error
	if rf, ok := ret.Get(0).(func(common.Color, time.Duration) error); ok {
		r0 = rf(color, duration)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}
