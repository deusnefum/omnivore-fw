package ppm

import (
	"machine"
	"math"
	"time"
)

const (
	// minimum number of miliseconds between a PPM frame
	minimumMSBetweenFrames = 6 * time.Millisecond
	// width in ms of a pulse to indicate zero point; theoretical is 1500, measured is 1525
	msMidpoint = 1500 * time.Microsecond
	// +/- ms to indicate range theoretical is 500, measured is closer to 550, but we want to keep this
	// small so the actual range can be measured/found automatically
	halfRange = 500 * time.Microsecond

	defaultEMAWindow = 4

	defaultDeadZoneThreshold = 0.12
)

// Input Shaping
type Shaping int

const (
	// Linear shaping passes the input more or less as-is; The deadZoneThreshold is implemented and automatic range is determined
	Linear Shaping = iota
	// Square shaping means the input value is squared. The effect of this is to give more fine-control near the zero-point, but still
	// allows for full-speed at maximums
	Square
	// Logarithmic is the opposite of square. Differences near zero are diminished, differences near channel maximums are increased.
	Logarithmic
	// Trinary is good for on/off buttons and mode switches. It ensures the Channel output is always exactly -1, 0, or 1.
	Trinary
)

type PPM struct {
	channels       [16]channel
	pin            machine.Pin
	currentChannel int
	lastChange     time.Time
}

type channel struct {
	deadZoneThreshold float64
	minMeasured       time.Duration
	maxMeasured       time.Duration
	actual            time.Duration
	emaWindow         float64
	value             float64
	shaping           Shaping
}

func New(pin machine.Pin) *PPM {
	pin.Configure(machine.PinConfig{Mode: machine.PinInputPulldown})
	p := &PPM{
		pin: pin,
	}
	// setup defaults for the channels
	for i := range p.channels {
		p.channels[i].value = 0
		p.channels[i].minMeasured = (msMidpoint - halfRange)
		p.channels[i].maxMeasured = (msMidpoint + halfRange)
		p.channels[i].deadZoneThreshold = defaultDeadZoneThreshold
		p.channels[i].emaWindow = defaultEMAWindow
		p.channels[i].shaping = Linear
	}
	return p
}

func (p *PPM) Start() {
	p.pin.SetInterrupt(machine.PinFalling, func(interruptPin machine.Pin) {
		t := time.Now()
		timeDiff := t.Sub(p.lastChange)
		p.lastChange = t
		if timeDiff > time.Duration(minimumMSBetweenFrames) {
			// start of PPM frame
			p.currentChannel = 0
			return
		}

		// shouldn't happen, but if we somehow found ourselves in this situation, do not modify any values
		if p.currentChannel > len(p.channels)-1 {
			return
		}

		//setMinMax(&(p.CurrentCh().minMeasured), &(p.CurrentCh().maxMeasured), timeDiff)

		p.CurrentCh().actual = timeDiff
		//p.CurrentCh().pushValue(timeDiff)
		// magic constants just fucking work better, okay?!
		p.CurrentCh().value = (float64(timeDiff)*2 - 3000000) / 1000000
		p.currentChannel++
	})

}

func (p *PPM) SetDeadZoneThreshold(ch int, threshold float64) {
	p.channels[ch].deadZoneThreshold = threshold
}

func (p *PPM) SetShaping(ch int, shape Shaping) {
	p.channels[ch].shaping = shape
}

func (p *PPM) SetWindow(ch int, window float64) {
	p.channels[ch].emaWindow = window
}

func (p *PPM) Channels() channel {
}

func (p *PPM) Actual(ch int) int64 {
	return p.channels[ch].actual.Microseconds() - 1500
}

func (p *PPM) Channel(ch int) float64 {
	// for SOME FUCKING REASON trying to do the float calculation IN the interrupt
	// breaks shit. Maybe because floating point math is hard? Acutally more like, because
	// with TinyGo, floating point math is soft. Get it? because the FPU doesn't work?
	scaled := (float64(p.channels[ch].actual.Microseconds())*2 - 3000) / 1000
	if math.Abs(scaled) < p.channels[ch].deadZoneThreshold {
		return 0
	}
	switch p.channels[ch].shaping {
	case Trinary:
		if scaled < -0.33 {
			return -1
		}
		if scaled > 0.33 {
			return 1
		}
		return 0
	case Square:
		if scaled > 0 {
			return scaled * scaled
		}
		return -scaled * scaled
	case Logarithmic:
		if scaled > 0 {
			return math.Log(20*scaled+1) / math.Log(21)
		}
		return -math.Log(20*-scaled+1) / math.Log(21)

	}
	// Default / Linear
	return scaled

	/*
		if math.Abs(p.channels[ch].value) < p.channels[ch].deadZoneThreshold {
			return 0
		}
		//return p.channels[ch].value
		switch p.channels[ch].shaping {
		case Trinary:
			if p.channels[ch].value < -0.33 {
				return -1
			}
			if p.channels[ch].value > 0.33 {
				return 1
			}
			return 0
		case Square:
			if p.channels[ch].value > 0 {
				return p.channels[ch].value * p.channels[ch].value
			}
			return -p.channels[ch].value * p.channels[ch].value
		case Logarithmic:
			if p.channels[ch].value > 0 {
				return math.Log(20*p.channels[ch].value+1) / math.Log(21)
			}
			return -math.Log(20*-p.channels[ch].value+1) / math.Log(21)

		}
		// Default / Linear
		return p.channels[ch].value
	*/
}

func (p *PPM) Stop() {
	// p.pin.SetInterrupt(0, nil)
}

func (p *PPM) CurrentCh() *channel {
	return &p.channels[p.currentChannel]
}

func (c *channel) pushValue(v time.Duration) {
	// disable EMA for now because nothing's working and this is just confusing me
	c.value = float64(v*2-c.maxMeasured-c.minMeasured) / float64(c.maxMeasured-c.minMeasured)
	return
	c.value = EMA(c.emaWindow,
		c.value,
		float64(v*2-c.maxMeasured-c.minMeasured)/float64(c.maxMeasured-c.minMeasured))
}

func EMA(window, prev, cur float64) float64 {
	const smoothing = 2
	return prev*(1-(smoothing/(1+window))) + cur*(smoothing/(1+window))
}

func Min(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

func Max(a, b time.Duration) time.Duration {
	if a > b {
		return a
	}
	return b
}

func setMinMax(minDest, maxDest *time.Duration, src time.Duration) {
	if src < *minDest {
		*minDest = src
		return
	}
	if src > *maxDest {
		*maxDest = src
	}
}

func setMin(dest *time.Duration, src time.Duration) {
	if src < *dest {
		*dest = src
	}
}

func setMax(dest *time.Duration, src time.Duration) {
	if src > *dest {
		*dest = src
	}
}
