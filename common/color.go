package common

// Color is used to represent the color and color temperature of a light.
// The color is represented as an HSB (Hue, Saturation, Brightness) value.
// The color temperature is represented in K (Kelvin) and is used to adjust the warmness / coolness of a white light, which is most obvious when saturation is close zero.
type Color struct {
	Hue        uint16 // range 0 to 65535
	Saturation uint16 // range 0 to 65535
	Brightness uint16 // range 0 to 65535
	Kelvin     uint16 // range 2500° (warm) to 9000° (cool)
}
