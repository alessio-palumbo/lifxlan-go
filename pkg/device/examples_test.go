package device_test

import (
	"fmt"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
)

func ExampleResolveSelectorSerials() {
	devices := []device.Device{
		{
			Serial:   mustSerial("001122334455"),
			Label:    "Desk",
			Group:    "Office",
			Location: "Home",
		},
		{
			Serial:   mustSerial("aabbccddeeff"),
			Label:    "Shelf",
			Group:    "Office",
			Location: "Home",
		},
		{
			Serial:   mustSerial("112233445566"),
			Label:    "TV",
			Group:    "Lounge",
			Location: "Home",
		},
	}

	for _, serial := range device.ResolveSelectorSerials("office, tv", devices) {
		fmt.Println(serial)
	}

	// Output:
	// 001122334455
	// aabbccddeeff
	// 112233445566
}

func ExampleClampBrightness() {
	fmt.Println(device.ClampBrightness(-5))
	fmt.Println(device.ClampVisibleBrightness(0))
	fmt.Println(device.ScaleBrightness(80, 0.5))

	// Output:
	// 0
	// 1
	// 40
}

func mustSerial(value string) device.Serial {
	serial, err := device.SerialFromHex(value)
	if err != nil {
		panic(err)
	}
	return serial
}
