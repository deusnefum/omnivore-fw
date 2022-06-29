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
	Channels       [16]Channel
	pin            machine.Pin
	currentChannel int
	lastChange     time.Time
}

type Channel struct {
	DeadZoneThreshold float64
	minMeasured       time.Duration
	maxMeasured       time.Duration
	Value             time.Duration
	Shaping           Shaping
}

func New(pin machine.Pin) *PPM {
	pin.Configure(machine.PinConfig{Mode: machine.PinInputPulldown})
	p := &PPM{
		pin: pin,
	}
	// setup defaults for the channels
	for i := range p.Channels {
		p.Channels[i].Value = 0
		p.Channels[i].minMeasured = (msMidpoint - halfRange)
		p.Channels[i].maxMeasured = (msMidpoint + halfRange)
		p.Channels[i].DeadZoneThreshold = defaultDeadZoneThreshold
		p.Channels[i].Shaping = Linear
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
		if p.currentChannel > len(p.Channels)-1 {
			return
		}

		// We could enable auto-scaling, but my particlar RC TX/RX seems *very* stable
		//setMinMax(&(p.CurrentCh().minMeasured), &(p.CurrentCh().maxMeasured), timeDiff)

		// just save the value; we can do that slow soft float work elsewhere
		p.Channels[p.currentChannel].Value = timeDiff
		p.currentChannel++
	})
}

// PulseDuration returns the actual time for the given channel in nanoseconds of the PPM pulse
func (p *PPM) PulseDuration(ch int) time.Duration {
	return p.Channels[ch].Value
}

func (p *PPM) Channel(ch int) float64 {
	scaled := (float64(p.Channels[ch].Value)*2 - 3000000) / 1000000
	if math.Abs(scaled) < p.Channels[ch].DeadZoneThreshold {
		return 0
	}
	switch p.Channels[ch].Shaping {
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
}

func (p *PPM) Stop() {
	p.pin.SetInterrupt(0, nil)
}

func (p *PPM) CurrentCh() *Channel {
	return &p.Channels[p.currentChannel]
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
