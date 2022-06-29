package main

import (
	"fmt"
	"machine"
	"math"
	"ppm"
	"time"
)

// Pinsssss
const (
	weaponPin = machine.GPIO27
	RC        = machine.GPIO5
)

// RC Channels
const (
	weaponModeChannel = 4
	weaponFireChannel = 5
	xAxisChannel      = 0
	yAxisChannel      = 1
	rotAxisChannel    = 3
	driveModeChannel  = 6
)

type weaponState int

const (
	weaponReady weaponState = iota
	weaponFiring
	weaponCharging
)

var (
	motor [4]*StepperMotor
	RCPPM *ppm.PPM
)

type weaponControl struct {
	pin       machine.Pin
	timestamp time.Time
	state     weaponState
}

var weapon = &weaponControl{}

// setup stepper motor control
func initMotors() {
	motor[0] = NewStepperMotor(machine.GPIO25, machine.GPIO15)
	motor[1] = NewStepperMotor(machine.GPIO16, machine.GPIO17)
	motor[2] = NewStepperMotor(machine.GPIO18, machine.GPIO19)
	motor[3] = NewStepperMotor(machine.GPIO20, machine.GPIO21)

	for i := range motor {
		motor[i].InitMotor()
	}
}

// setup reading from RC receiver
func initRC() {
	RCPPM = ppm.New(RC)
	RCPPM.Start()
}

func initIMU() {

}

func waitHere() {
	fmt.Println("waitHere")
	for {
		time.Sleep(time.Hour)
	}
}

func (w *weaponControl) init() {
	weapon.pin = weaponPin
	weapon.pin.Configure(machine.PinConfig{Mode: machine.PinOutput})

	// Use trinary shaping for both wepaon mode and fire button this ensures we get
	// a -1, 0, or 1 from the RCPPM.Channel() call
	RCPPM.Channels[weaponModeChannel].Shaping = ppm.Trinary
	RCPPM.Channels[weaponFireChannel].Shaping = ppm.Trinary
}

func init() {
	machine.UART0.Configure(machine.UARTConfig{
		TX:       machine.GPIO0,
		RX:       machine.GPIO1,
		BaudRate: 115200,
	})

	machine.GPIO26.Configure(machine.PinConfig{Mode: machine.PinOutput})
	machine.LED.Configure(machine.PinConfig{Mode: machine.PinOutput})
}

func main() {
	initRC()
	//initIMU()
	initMotors()
	weapon.init()

	for {
		weapon.inputLoop()
		motorControl()
		time.Sleep(time.Duration(100 * time.Microsecond))
	}
}

func motorControl() {
	m := sineDrive(RCPPM.Channel(xAxisChannel), RCPPM.Channel(yAxisChannel), RCPPM.Channel(rotAxisChannel))
	fmt.Printf("x: %+1.2f y: %+1.2f r: %+1.2f; m0: %+1.2f m1: %+1.2f m2: %+1.2f m3: %+1.2f\r\n", RCPPM.Channel(0), RCPPM.Channel(1), RCPPM.Channel(rotAxisChannel), m[0], m[1], m[2], m[3])
	motor[0].Set(m[0])
	motor[1].Set(m[1])
	motor[2].Set(m[2])
	motor[3].Set(m[3])
}

func (w *weaponControl) SetState(state weaponState) {
	w.state = state
	w.timestamp = time.Now()
}

func (w *weaponControl) inputLoop() {
	if RCPPM.Channel(weaponModeChannel) == 0 {
		return
	}

	switch w.state {
	case weaponReady:
		if RCPPM.Channel(weaponFireChannel) == 1 || (RCPPM.Channel(weaponModeChannel) == 1 && false) {
			w.pin.High()
			machine.GPIO26.High()
			w.SetState(weaponFiring)

		}
	case weaponFiring:
		if time.Since(w.timestamp) > 240*time.Millisecond {
			w.pin.Low()
			machine.GPIO26.Low()
			w.SetState(weaponCharging)
		}
	}

	if time.Since(w.timestamp) > 2*time.Second {
		w.SetState(weaponReady)
	}
}

// a generic version of this would be more useful, but surely the hard-coded 4 motor
// version is faster. Genuinely don't remember how this math works, but I think it
// does work so 🤷
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
