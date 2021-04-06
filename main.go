package main

import (
	"machine"

	"dshot"
)

// Pinsssss
const (
	ESC1 = machine.D2
	ESC2 = machine.D3
	ESC3 = machine.D4
	ESC4 = machine.D5

	WeaponSensor = machine.D12
	Weapon       = machine.D7

	// Compass I2C
)

func main() {
	// Setup weapon sensor to use interrupt
	WeaponSensor.Configure(machine.PinConfig{Mode: machine.PinInput})

	// setup weapon firing pin
	Weapon.Configure(machine.PinConfig{Mode: machine.PinOutput})

	// setup compass

	// initialize our ESC outputs
	dshot.InitPin(ESC1)
	dshot.InitPin(ESC2)
	dshot.InitPin(ESC3)
	dshot.InitPin(ESC4)

	// initalize dshot600
	ds := dshot.NewDShot(600)

	// start and arm ESCs
	escCmdChan, _ := ds.Start(ESC1)

	// launch goproc to monitor compass and correct heading

	for {
	}
}
