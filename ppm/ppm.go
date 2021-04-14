package ppm

import (
	"machine"
	"time"
)

type PPM struct {
	pin            machine.Pin
	Channels       [16]float64
	currentChannel int
	lastChange     time.Time
}

func New(pin machine.Pin) *PPM {
	pin.Configure(machine.PinConfig{Mode: machine.PinInputPulldown})
	return &PPM{
		pin: pin,
	}
}

func (p *PPM) Start() {
	p.pin.SetInterrupt(machine.PinRising, func(pin machine.Pin) {
		t := time.Now()
		timeDiff := t.Sub(p.lastChange)
		p.lastChange = t
		if timeDiff > time.Duration(3*time.Millisecond) {
			// start of PPM frame
			p.currentChannel = 0
			return
		}

		if p.currentChannel > len(p.Channels) {
			return
		}

		p.Channels[p.currentChannel] = float64(timeDiff.Microseconds()-1500) / 500.0
		p.currentChannel++
	})

}

func (p *PPM) Stop() {
	p.pin.SetInterrupt(0, nil)
}
