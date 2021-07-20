package dshot

/* This is mostly based off of https://www.swallenhardware.io/battlebots/2019/4/20/a-developers-guide-to-dshot-escs

To change the spin direction, set 3D mode, or save settings you must enable the telemetry bit in the associated command packet, and you must issue the command 10 times in a row for the command to take effect.

*/

import (
	"machine"
	"math"
	"time"
)

const (
	CmdMotorStop = iota
	CmdBeacon1
	CmdBeacon2
	CmdBeacon3
	CmdBeacon4
	CmdBeacon5
	CmdESCInfo // v2 includes settings
	CmdSpinDirection1
	CmdSpinDirection2
	Cmd3DModeOff
	Cmd3DModeOn
	CmdSettingsRequest // not implemented
	CmdSaveSettings
	CmdSpinDirectionNormal
	CmdSpinDirectionReversed
	CmdLED0On               // BLHeli32 only
	CmdLED1On               // BLHeli32 only
	CmdLED2On               // BLHeli32 only
	CmdLED3On               // BLHeli32 only
	CmdLED0Off              // BLHeli32 only
	CmdLED1Off              // BLHeli32 only
	CmdLED2Off              // BLHeli32 only
	CmdLED3Off              // BLHeli32 only
	CmdAudioStreamModeOnOff // KISS audio Stream mode on/off
	CmdSilentModeOnOff      // KISS silent mode on/off
	CmdSignalLineTelemetryDisable
	CmdSignalLineContinuousERPMTelemetry
	CmdMax = 47
)

const (
	// queueSize is number of throttle / dshot commands to queue before
	// making program-flow wait
	queueSize = 1
)

// DShot defines the use of the dshot protocol, mostly this is for tracking
// what protocol speed to use.
type DShot struct {
	speed uint

	// bits contain timing information for sending 1s and 0s via the dshot protocol
	// bit[0] are the timings for sending a zero
	// bit[1] are the timings for sending a one
	// bit[x][0] is the line-high time
	// bit[x][1] is the line-low time
	bits bits
}

// Frame defines a dshot frame
type Frame struct {
	// Throttle is the throttle value or command to send to the ESC
	Throttle uint16
	// Telemetry determine whether or not to set the telemetry bit (I don't know what that does)
	Telemetry bool
}

// DShotChannel
type Channel struct {
	Cmd    chan Frame
	Cancel chan struct{}

	// since Cmd 0 (motor stop) acts as a toggle, we need to track what we've sent
	// to know if the motor is spinning or not
	spinning bool
}

type bitTime [2]time.Duration
type bits [2]bitTime

// NewDShot returns an initialized DShot struct
func NewDShot(speed uint) *DShot {
	ds := &DShot{speed: speed}

	switch speed {
	case 150:
		ds.bits[1][0] = 4990
		ds.bits[1][1] = 1677

		ds.bits[0][0] = 2495
		ds.bits[0][1] = 4172

	case 300:
		ds.bits[1][0] = 2495
		ds.bits[1][1] = 838

		ds.bits[0][0] = 1248
		ds.bits[0][1] = 2086

	case 600:
		ds.bits[1][0] = 1248
		ds.bits[1][1] = 419

		ds.bits[0][0] = 624
		ds.bits[0][1] = 1043

	case 1200:
		ds.bits[1][0] = 624
		ds.bits[1][1] = 210

		ds.bits[0][0] = 312
		ds.bits[0][1] = 521
	default:
		panic("incorrect dshot speed used")
	}

	return ds
}

// SendFrame takes a Frame and pre-configured machine.Pin and transmits
// the frame on the pin (bit-bang)
func (ds *DShot) SendFrame(dsf *Frame, pin machine.Pin) {
	// Send MSB first
	// Send LSB first?!?!
	data := dsf.encode()
	for i := 15; i >= 0; i-- {
		bMasked := int((data >> i) & 1)
		pin.High()
		time.Sleep(ds.bits[bMasked][0])
		pin.Low()
		time.Sleep(ds.bits[bMasked][1])
	}
	time.Sleep(time.Duration(50) * time.Microsecond)
	// leave line low
}

// RepeatSendFrame calls SendFrame n-times. Basically anything other than
// changing throttle speed requires a command to be repeated.
func (ds *DShot) RepeatSendFrame(n int, dsf *Frame, pin machine.Pin) {
	data := dsf.encode()
	for j := 0; j < n; j++ {
		// Send MSB first
		for i := 15; i >= 0; i-- {
			bMasked := (data >> i) & 1
			pin.High()
			time.Sleep(ds.bits[bMasked][0])
			pin.Low()
			time.Sleep(ds.bits[bMasked][1])
		}
		time.Sleep(time.Duration(20) * time.Microsecond)
	}
}

// InitPin initializes a machine.Pin for communicating with an ESC
func InitPin(pin machine.Pin) {
	pin.Configure(machine.PinConfig{Mode: machine.PinOutput})
}

func (df *Frame) encode() (frame uint16) {
	frame = (df.Throttle << 1)
	if df.Telemetry {
		frame |= 1
	}

	// calc checksum
	var csum uint16
	/*
		csumData := frame
		for i := 0; i < 3; i++ { // tinygo/LLVM is smart enough to unroll this, right?
			csum ^= csumData // xor data by nibbles
			csumData >>= 4
		}
		csum &= 0x000f
	*/
	csum = (frame ^ (frame >> 4) ^ (frame >> 8)) & 0x0F

	// append checksum
	return (frame << 4) | csum
}

// NewChannel will start dshot protocol on the given pin, returning a Channel
func (ds *DShot) NewChannel(pin machine.Pin) *Channel {
	ch := &Channel{}
	ch.Cmd = make(chan Frame, queueSize)
	ch.Cancel = make(chan struct{})
	ch.spinning = false

	go func() {
		// lastSend := time.Now()
		// throttleFrame := Frame{}
		for {
			select {
			case newFrame := <-ch.Cmd:
				// if we got a throttle, update throttle
				/*if newFrame.Throttle > CmdMax {
					throttleFrame = newFrame
				}*/
				ds.SendFrame(&newFrame, pin)
				// might need a brief pause here
				//lastSend = time.Now()
			case <-ch.Cancel:
				// cancel signal causes this go func to return
				// any shutdown commands need to be sent prior to
				// calling this
				return
				/*
					default: // default makes this a non-blocking operation
						// don't spin too hard here
						time.Sleep(time.Millisecond)
						// dshot ESCs will cut power to a motor if it doesn't get a frame
						// every ~10ms. So, repeat the last throttle frame sent
						if time.Since(lastSend) > time.Duration(8*time.Millisecond) {
							if throttleFrame.Throttle > CmdMax {
								ds.SendFrame(&throttleFrame, pin)
								lastSend = time.Now()
							}
						}
				*/
			}
		}
	}()

	return ch
}

func (ch *Channel) Stop() {
	// do other shutdown here?
	ch.Cancel <- struct{}{}
}

// Set3DThrottle sets the throttle speed, based on a float64. -1 is full reverse; 1 is full forward
// and 0 is stopped
func (ch *Channel) Set3DThrottle(speed float64) {
	var throttle uint16
	// if we are spinning, send the motor stop command to actually stop
	if math.Abs(speed) < 0.001 && ch.spinning {
		ch.SendCmd(CmdMotorStop, 1)
		ch.spinning = false
	} else if math.Abs(speed) >= 0.001 && !ch.spinning {
		ch.SendCmd(CmdMotorStop, 1)
		ch.spinning = true
	}

	// I'm assuming here "direction1" is reverse and direction2 is forward, might need to
	// change the below less-than 0 to greater-than 0
	if speed < 0 {
		throttle = uint16(speed*999) + 48
	} else {
		throttle = uint16(speed*998) + 1048
	}

	ch.SendCmd(throttle, 1)
}

func (ch *Channel) SendFrame(f *Frame) {
	ch.Cmd <- *f
}

func (ch *Channel) SendCmd(cmd uint16, repeat int) {
	f := &Frame{
		Throttle:  cmd,
		Telemetry: true,
	}
	for i := 0; i < repeat; i++ {
		ch.Cmd <- *f
	}
}

func (ch *Channel) Init() {
}

func spin(duration time.Duration) {
	start := time.Now()
	for time.Since(start) < duration {
	}
}
