package ppm

import (
	"machine"
	"math"
	"time"
)

type PPM struct {
	pin               machine.Pin
	Channels          [16]float64
	currentChannel    int
	lastChange        time.Time
	DeadZoneThreshold float64
}

func New(pin machine.Pin) *PPM {
	pin.Configure(machine.PinConfig{Mode: machine.PinInputPulldown})
	return &PPM{
		pin:               pin,
		DeadZoneThreshold: 0.15,
	}
}

func (p *PPM) Start() {
	p.pin.SetInterrupt(machine.PinFalling, func(interruptPin machine.Pin) {
		t := time.Now()
		timeDiff := t.Sub(p.lastChange)
		p.lastChange = t
		if timeDiff > time.Duration(6*time.Millisecond) {
			// start of PPM frame
			p.currentChannel = 0
			return
		}

		if p.currentChannel > len(p.Channels)-1 {
			return
		}

		// theoretically, the range is between 1ms and 2ms, however in practice with the particular
		// receiver I'm using, the upper end and lower end range more than this, hence we divide by
		// 585 instead of 500.
		// 975 : 2075

		//p.Channels[p.currentChannel] = ((math.Round(float64(timeDiff.Microseconds()) / 100)) - 15) / 5.5
		p.Channels[p.currentChannel] = (p.Channels[p.currentChannel] + ((float64(timeDiff.Microseconds()) - 1525) / 550)) / 2
		p.currentChannel++
	})

}

func (p *PPM) Channel(ch int) float64 {
	if math.Abs(p.Channels[ch]) < p.DeadZoneThreshold {
		return 0
	}
	return p.Channels[ch]
}

func (p *PPM) Stop() {
	p.pin.SetInterrupt(0, nil)
}
