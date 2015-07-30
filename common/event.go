package common

// EventNewDevice is emitted by a Client when it discovers a new Device
type EventNewDevice struct {
	Device Device
}

// EventExpiredDevice is emitted by a Client when a Device is no longer known
type EventExpiredDevice struct {
	Device Device
}

// EventUpdateLabel is emitted by a Device when it's label is updated
type EventUpdateLabel struct {
	Label string
}

// EventUpdatePower is emitted by a Device when it's power state is updated
type EventUpdatePower struct {
	Power bool
}

// EventUpdateColor is emitted by a Light when it's Color is updated
type EventUpdateColor struct {
	Color Color
}
