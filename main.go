package main

import (
	"dshot"
	"fmt"
	"machine"
	"math"
	"time"

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

	PressureSensorPin = machine.A0

	RC = machine.D10

	// what each RC channel is for
	weaponModeChannel = 4
	weaponFireChannel = 5
	xAxisChannel      = 0
	yAxisChannel      = 1
	driveModeChannel  = 6
)

var (
	PressureSensor machine.ADC
	compass        lis2mdl.Device
	heading        float64
	RCPPM          *ppm.PPM
)

func log(msg string) {
	println(fmt.Sprintf("%d %s", time.Now().Unix(), msg))
}

func main() {
	// Need ADC to read pressure sensor voltage
	machine.InitADC()
	PressureSensorPin.Configure(machine.PinConfig{Mode: machine.PinAnalog})
	PressureSensor = machine.ADC{PressureSensorPin}
	PressureSensor.Configure(machine.ADCConfig{})

	// we boot too fast to catch start up log messages, so sleep a couple seconds
	// to get the UART0 / usb connection up
	time.Sleep(time.Duration(2 * time.Second))

	// Setup weapon sensor to use interrupt
	WeaponSensor.Configure(machine.PinConfig{Mode: machine.PinInputPullup})
	//WeaponSensor.SetInterrupt(machine.PinFalling, fireWeapon)

	// setup weapon firing pin
	Weapon.Configure(machine.PinConfig{Mode: machine.PinOutput})

	// setup reading from RC receiver
	RCPPM = ppm.New(RC)
	RCPPM.Start()
	// setup compass
	// set up i2c first
	machine.I2C0.Configure(machine.I2CConfig{})
	// instantiate lis2mdl
	compass = lis2mdl.New(machine.I2C0)
	compass.Configure(lis2mdl.Configuration{})

	// initialize our ESC outputs
	ds := dshot.NewDShot(600)
	dshot.InitPin(ESC1)
	dshot.InitPin(ESC2)
	dshot.InitPin(ESC3)
	dshot.InitPin(ESC4)

	// initialize dshot600
	//ds := dshot.NewDShot(600)

	// start and arm ESCs
	escCmdChan0, _ := ds.Start(ESC1)
	escCmdChan1, _ := ds.Start(ESC2)
	escCmdChan2, _ := ds.Start(ESC3)
	escCmdChan3, _ := ds.Start(ESC4)

	// for {
	// print(fmt.Sprintf("Heading: %+1.3f\r", heading))
	// // print(fmt.Sprintf("weaponMode: %+1.3f\r", RCPPM.Channel(weaponModeChannel)))
	// time.Sleep(time.Duration(100) * time.Millisecond)
	// }

	go updateHeading()
	go weaponControl()

	f := [4]dshot.Frame{}
	forward := [4]bool{}
	for {
		// var heading float64

		// very simplistic heading correction algo
		// will need to refine this once I have some motors hooked up

		// FAILSAFE:
		// check RCPPM.Channel(driveModeChannel) and if it's 0 for more than 30 seconds or so, disarm the ESCs

		var rotation float64
		if RCPPM.Channel(driveModeChannel) > 0.6 && compass.Connected() {
			// rotation = getHeading() / -180
			rotation = heading / 180
			rotation *= math.Abs(rotation)
			if math.Abs(rotation) < 0.05 {
				rotation = 0
			}
		}
		// manual rotation control works regardless of auto-drive switch
		if manualRot := RCPPM.Channel(3); manualRot != 0 {
			rotation = manualRot
		}

		m := sineDrive(RCPPM.Channel(xAxisChannel), RCPPM.Channel(yAxisChannel), rotation)

		print(fmt.Sprintf("x: %+1.2f y: %+1.2f r: %+1.2f; m0: %+1.2f m1: %+1.2f m2: %+1.2f m3: %+1.2f\r", RCPPM.Channel(0), RCPPM.Channel(1), rotation, m[0], m[1], m[2], m[3]))

		for i := range m {
			if m[i] >= 0 && !forward[i] {
				f[i].Throttle = dshot.CmdSpinDirectionNormal
				forward[i] = true
				continue
			}
			if m[i] < 0 && forward[i] {
				f[i].Throttle = dshot.CmdSpinDirectionReversed
				forward[i] = false
				continue
			}

			f[i].Throttle = uint16(math.Abs(m[i])*2000 + dshot.CmdMax)
		}

		escCmdChan0 <- f[0]
		escCmdChan1 <- f[1]
		escCmdChan2 <- f[2]
		escCmdChan3 <- f[3]

		// dshot600 ESCs can't update faster than ~28 microseconds, so no need to loop faster than that
		// and if we loop *too* fast we'll end up blocking up the control channels, so slightly slower is better
		time.Sleep(time.Duration(30 * time.Microsecond))
	}
}

func weaponControl() {
	weaponFireTime := time.Time{}
	// can do Pressure Sensor averaging and auto-calibration here as well
	// calibrate pressure sensor
	var calibrated uint32
	for i := 0; i < 10; i++ {
		calibrated += uint32(PressureSensor.Get())
	}
	calibrated /= 10

	for {
		time.Sleep(time.Duration(10) * time.Millisecond)
		// if in manual or auto mode; otherwise weapon is disabled / off
		mode := RCPPM.Channel(weaponModeChannel)
		// when the receiver loses signal, this channel defaults to 0, so for safety reasons
		// we use 0 to mean "off"
		switch {
		case mode < 0.5 && mode > -0.5:
			continue
		case RCPPM.Channel(weaponFireChannel) > 0.5:
			fallthrough
		case uint32(PressureSensor.Get()) > calibrated*2:
			fallthrough
		case WeaponSensor.Get() && mode < -0.5:
			if time.Since(weaponFireTime) < time.Duration(500*time.Millisecond) {
				continue
			}
			println("BOOM!")
			Weapon.High()
			time.Sleep(time.Duration(200 * time.Millisecond))
			Weapon.Low()
			weaponFireTime = time.Now()
		}
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
	dV := math.Min(math.Sqrt(y*y+x*x), 1)

	if (x == 0 && y == 0) && (dV+rotation) == 0 {
		return [4]float64{0, 0, 0, 0}
	}

	motorPositions := [4]float64{math.Pi * 3 / 4, math.Pi / 4, math.Pi * 7 / 4, math.Pi * 5 / 4}

	pV := dV + math.Abs(rotation)
	rotation = rotation * math.Abs(rotation/pV)
	dV = dV * dV / pV

	for i, offset := range motorPositions {
		out[i] = math.Sqrt2 * (dV*-math.Sin(dTheta-offset) + rotation)
		out[i] = math.Min(math.Max(out[i], -1), 1)
	}

	//print(fmt.Sprintf("dV: %+1.3f dTheta: %+1.3f %+v\r", dV, dTheta, out))
	return
}

func debug() {
	var prevHeading float64
	for {
		if compass.Connected() {
			heading := float64(compass.ReadCompass())
			// if heading > 180 {
			// heading -= 360
			// }

			// prevHeading = (prevHeading*9 + heading) / 10
			prevHeading = heading
			print(fmt.Sprintf("%10d %+3.0f %+3.0f\n", time.Now().Unix(), heading, math.Round(prevHeading)))
		}

		// for i := 0; i < 8; i++ {
		// print(fmt.Sprintf("CH%d: %+1.2f ", i+1, RCPPM.Channels[i]))
		// }
		// print("\n")

		m := sineDrive(RCPPM.Channel(0), RCPPM.Channel(1), 0)
		print(fmt.Sprintf("M0:%+1.3f M1:%+1.3f M2:%+1.3f M3:%+1.3f\n", m[0], m[1], m[2], m[3]))

		//println(fmt.Sprintf("%+1.3f, %+1.3f", RCPPM.Channel(0), RCPPM.Channel(1))

		time.Sleep(time.Duration(1000 * time.Millisecond))
	}
}

func getHeading() float64 {
	x, y, _ := compass.ReadMagneticField()
	xf, yf := float64(x)*0.15, float64(y)*0.15
	// if we swap x and y, we basically get what we expect, 0
	// means we're facing north on the y axis
	return (math.Atan2(yf, xf) * 180) / math.Pi
}

func updateHeading() {
	for {
		x, y, _ := compass.ReadMagneticField()
		xf, yf := float64(x)*0.15, float64(y)*0.15
		newHeading := (math.Atan2(xf, yf) * 180) / math.Pi

		heading = angleAvg(heading, angleAvg(heading, newHeading))

		time.Sleep(15 * time.Millisecond)
	}
}

func angleAvg(a, b float64) float64 {
	diff := math.Abs(a - b)
	if diff < 180 {
		return (a + b) / 2
	}
	diff = 360 - diff
	out := math.Min(a, b) - diff/2
	if out < -180 {
		return 360 + out
	}
	return out
}
