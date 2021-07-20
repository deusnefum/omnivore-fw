package main

import (
	"fmt"
	"machine"
	"math"
	"time"

	"ppm"

	"tinygo.org/x/drivers/lis2mdl"
)

// Pinsssss
const (
	ESC1 = machine.D13
	ESC2 = machine.D12
	ESC3 = machine.D11
	ESC4 = machine.D10

	WeaponSensor = machine.D12
	Weapon       = machine.D7

	PressureSensorPin = machine.A0

	RC = machine.D9

	// what each RC channel is for
	weaponModeChannel = 4
	weaponFireChannel = 5
	xAxisChannel      = 0
	yAxisChannel      = 1
	rotAxisChannel    = 3
	driveModeChannel  = 6
)

var (
	PressureSensor machine.ADC
	compass        lis2mdl.Device
	heading        float64
	RCPPM          *ppm.PPM
)

func log(msg string) {
	println(fmt.Sprintf("[%d] %s", time.Now().UnixNano(), msg))
}

func main() {
	// we boot too fast to catch start up log messages, so sleep a couple seconds
	// to get the UART0 / usb connection up
	time.Sleep(time.Duration(2 * time.Second))
	log("omnivore start")

	// Need ADC to read pressure sensor voltage
	machine.InitADC()
	PressureSensorPin.Configure(machine.PinConfig{Mode: machine.PinAnalog})
	PressureSensor = machine.ADC{PressureSensorPin}
	PressureSensor.Configure(machine.ADCConfig{})
	log("ADC configured")

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
	log("compass and rc/ppm configured")

	// start pwm for motors
	TCC0 := machine.TCC0
	var pwmChan [4]uint8
	for i, pin := range [4]machine.Pin{ESC1, ESC2, ESC3, ESC4} {
		var err error
		pwmChan[i], err = TCC0.Channel(pin)
		if err != nil {
			log("error configuring pwm channel: " + err.Error())
		}
		err = TCC0.Configure(machine.PWMConfig{Period: uint64(20 * time.Millisecond)})
		if err != nil {
			log("error configuring TCC0 PWM: " + err.Error())
		}
		TCC0.Set(pwmChan[i], TCC0.Top()/13)
	}
	log("pwm/esc started")

	go updateHeading()
	go weaponControl()

	for {
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
		if manualRot := RCPPM.Channel(rotAxisChannel); manualRot != 0 {
			rotation = manualRot
		}

		m := sineDrive(RCPPM.Channel(xAxisChannel), RCPPM.Channel(yAxisChannel), rotation)

		print(fmt.Sprintf("x: %+1.2f y: %+1.2f r: %+1.2f; m0: %+1.2f m1: %+1.2f m2: %+1.2f m3: %+1.2f\r", RCPPM.Channel(0), RCPPM.Channel(1), rotation, m[0], m[1], m[2], m[3]))

		// weaponControl()

		for i := range m {
			divisor := math.Round(40 / (m[i] + 3))
			TCC0.Set(pwmChan[i], TCC0.Top()/uint32(divisor))
		}

		time.Sleep(time.Duration(10 * time.Millisecond))
	}
}

func weaponControl() {
	log("weapon control starting")
	weaponFireTime := time.Time{}
	// can do Pressure Sensor averaging and auto-calibration here as well
	// calibrate pressure sensor
	var calibrated uint32
	for i := 0; i < 10; i++ {
		calibrated += uint32(PressureSensor.Get())
	}
	calibrated /= 10

	//log("entering weapon mainloop")
	for {
		time.Sleep(time.Duration(10 * time.Millisecond))
		// if in manual or auto mode; otherwise weapon is disabled / off
		mode := RCPPM.Channel(weaponModeChannel)
		// when the receiver loses signal, this channel defaults to 0, so for safety reasons
		// we use 0 to mean "off"
		switch {
		case mode < 0.5 && mode > -0.5:
			continue
			// return
		case RCPPM.Channel(weaponFireChannel) > 0.5:
			fallthrough
		case uint32(PressureSensor.Get()) > calibrated*2:
			fallthrough
		case WeaponSensor.Get() && mode < -0.5:
			if time.Since(weaponFireTime) < time.Duration(500*time.Millisecond) {
				continue
				// return
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
