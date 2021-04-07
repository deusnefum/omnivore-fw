package dshot

import (
	"machine"
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
	queueSize = 10
)

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
	crc       byte
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
		ds.bits[0][0] = 4172

	case 300:
		ds.bits[1][0] = 2495
		ds.bits[1][1] = 838

		ds.bits[0][0] = 1248
		ds.bits[0][0] = 2086

	case 600:
		ds.bits[1][0] = 1248
		ds.bits[1][1] = 419

		ds.bits[0][0] = 624
		ds.bits[0][0] = 1043

	case 1200:
		ds.bits[1][0] = 624
		ds.bits[1][1] = 210

		ds.bits[0][0] = 312
		ds.bits[0][0] = 521
	}

	return ds
}

// SendFrame taks a Frame and pre-configured machine.Pin and transmits
// the frame on the pin (bit-bang)
func (ds *DShot) SendFrame(dsf *Frame, pin machine.Pin) {
	// Send MSB first
	data := dsf.encode()
	for i := 0; i < 16; i++ {
		bMasked := data & 0x8000
		pin.High()
		time.Sleep(ds.bits[bMasked][0])
		pin.Low()
		time.Sleep(ds.bits[bMasked][1])
		data <<= 1
	}
	time.Sleep(time.Duration(20) * time.Microseconds)
	// go high here again??
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
	csumData := frame
	for i := 0; i < 3; i++ { // tinygo is smart enough to unroll this, right?
		csum ^= csumData // xor data by nibbles
		csumData >>= 4
	}
	csum &= 0xf
	// append checksum
	frame = (frame << 4) | csum

	return
}

// Start will start dshot protocol on the given pin, returning a cmd channel for
// feeding throttle/cmd frames to. The cancel struct will stop the go proc.
// This will arm (but should not spin up) the ESC
func (ds *DShot) Start(pin machine.Pin) (cmd chan Frame, cancel chan struct{}) {
	cmd = make(chan Frame, queueSize)
	cancel = make(chan struct{})

	// Arm sequence
	armFrame := &Frame{Throttle: CmdMax + 2}
	ds.SendFrame(armFrame, pin)
	armFrame.Throttle = CmdMax + 1
	ds.SendFrame(armFrame, pin)

	go func() {
		lastSend := time.Now()
		throttleFrame := Frame{}
		for {
			select {
			case newFrame := <-cmd:
				// if we got a throttle, update throttle
				if newFrame.Throttle > CmdMax {
					throttleFrame = newFrame
				}
				ds.SendFrame(&newFrame, pin)
				// might need a brief pause here
				lastSend = time.Now()
			case <-cancel:
				// whoever sent us a cancel signal should close the channels
				// send signal to stop motor
				ds.SendFrame(&Frame{Throttle: CmdMax + 1}, pin)
				ds.SendFrame(&Frame{Throttle: CmdMotorStop}, pin)
				return
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
			}

		}
	}()

	return cmd, cancel
}
