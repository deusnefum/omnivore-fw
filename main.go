package main

import (
	"fmt"
	"machine"
	"math"
	"time"

	"dshot"
	"ppm"

	"tinygo.org/x/drivers/lis2mdl"
)

// Pinsssss
const (
	ESC1 = machine.D2
	ESC2 = machine.D3
	ESC3 = machine.D4
	ESC4 = machine.D5

	WeaponSensor = machine.D12
	Weapon       = machine.D7

	RC = machine.D10
)

var (
	compass lis2mdl.Device
	RCPPM   *ppm.PPM
)

func log(msg string) {
	println(fmt.Sprintf("%d %s", time.Now().Unix(), msg))
}

func main() {
	// we boot too fast to catch start up log messages, so sleep a couple seconds
	// to get the UART0 / usb connection up
	time.Sleep(time.Duration(2 * time.Second))

	// Setup weapon sensor to use interrupt
	WeaponSensor.Configure(machine.PinConfig{Mode: machine.PinInput})

	// setup weapon firing pin
	Weapon.Configure(machine.PinConfig{Mode: machine.PinOutput})

	// setup reading from RC receiver
	RCPPM = ppm.New(RC)
	RCPPM.Start()

	// setup compass
	// set up i2c first
	machine.I2C0.Configure(machine.I2CConfig{})
	// instantiate lis2mdl
	compass := lis2mdl.New(machine.I2C0)
	compass.Configure(lis2mdl.Configuration{})
	if !compass.Connected() {
		log("Not connected to compass!")
	}

	var prevHeading float64
	for {
		heading := float64(compass.ReadCompass())
		if heading > 180 {
			heading -= 360
		}

		prevHeading = (prevHeading*9 + heading) / 10
		print(fmt.Sprintf("%10d %+3.0f %+3.0f"))
		for _, v := range RCPPM.Channels {
			print(fmt.Sprintf("%+1.2f ", v))
		}
		print("\r")

		time.Sleep(time.Duration(500 * time.Millisecond))
	}

	// initialize our ESC outputs
	dshot.InitPin(ESC1)
	dshot.InitPin(ESC2)
	dshot.InitPin(ESC3)
	dshot.InitPin(ESC4)

	// initialize dshot600
	//ds := dshot.NewDShot(600)

	// start and arm ESCs
	//escCmdChan, _ := ds.Start(ESC1)

	// launch goproc to monitor compass and correct heading

	for {
	}
}

func readReceiver() {
	// read PWM input from RC receiver, convert PWM (width varies from 1ms to 2ms)
	// into a float ranging from -1 to 1
	//rxRaw := make([]int, rxInputCount)
	for {

	}
}

// take x/y input from receiver, do trig, send signals to ESCs
func controlLoop() {
	for {
		// dshot600 ESCs can't update faster than ~28 microseconds, so no need to loop faster than that

		time.Sleep(time.Duration(28 * time.Microsecond))
	}
}

// given x, y, and theta, determine how much to move 4 motors (wheels)
/*
		 ^
		 y
		 v

	   0   1
		[ ]   <-- x -->
	   3   2
*/
func sineDrive(x, y, rotation float64) (out [4]float64) {
	// The desired angle of movement
	dTheta := math.Atan2(y, x)
	// magnitude of movement
	dV := math.Sqrt(y*y + x*x)

	motorPositions := [4]float64{math.Pi * 3 / 4, math.Pi / 4, math.Pi * 7 / 4, math.Pi * 5 / 4}

	for i, offset := range motorPositions {
		out[i] = dV/(dV+math.Abs(rotation))*-math.Sin(dTheta-offset) + rotation/(dV+math.Abs(rotation))
	}

	return
}
