package audio

import (
	"encoding/binary"
	"io"
	"math"
)

// combFilter is a feedback comb filter: y[n] = x[n] + g * y[n-D]
type combFilter struct {
	buf []float64
	pos int
	g   float64
}

func newCombFilter(delayMs, g float64) combFilter {
	d := int(delayMs * sampleRate / 1000)
	return combFilter{buf: make([]float64, d), g: g}
}

func (c *combFilter) process(x float64) float64 {
	out := x + c.g*c.buf[c.pos]
	c.buf[c.pos] = out
	c.pos = (c.pos + 1) % len(c.buf)
	return out
}

// allpassFilter is a Schroeder allpass section: diffuses echo density.
type allpassFilter struct {
	buf []float64
	pos int
	g   float64
}

func newAllpassFilter(delayMs, g float64) allpassFilter {
	d := int(delayMs * sampleRate / 1000)
	return allpassFilter{buf: make([]float64, d), g: g}
}

func (a *allpassFilter) process(x float64) float64 {
	delayed := a.buf[a.pos]
	v := x + a.g*delayed
	a.buf[a.pos] = v
	a.pos = (a.pos + 1) % len(a.buf)
	return delayed - a.g*v
}

// Reverb wraps an io.Reader with a Schroeder reverb (4 parallel combs + 2 series allpass).
// Delay times are incommensurate; g values are tuned for ~1s RT60 at 44100Hz.
// Wet controls the dry/wet mix (0=dry, 1=full reverb).
type Reverb struct {
	src   io.Reader
	tmp   []byte
	combs [4]combFilter
	ap1   allpassFilter
	ap2   allpassFilter
	Wet   float64
}

func NewReverb(src io.Reader, wet float64) *Reverb {
	return &Reverb{
		src: src,
		combs: [4]combFilter{
			newCombFilter(36.2, 0.773),
			newCombFilter(42.3, 0.742),
			newCombFilter(46.5, 0.721),
			newCombFilter(53.0, 0.698),
		},
		ap1: newAllpassFilter(6.3, 0.70),
		ap2: newAllpassFilter(2.1, 0.70),
		Wet: wet,
	}
}

func (r *Reverb) Read(buf []byte) (int, error) {
	if len(r.tmp) < len(buf) {
		r.tmp = make([]byte, len(buf))
	}
	n, err := r.src.Read(r.tmp[:len(buf)])
	n = n &^ 3 // align to 4-byte stereo sample boundary

	for i := 0; i < n; i += 4 {
		dry := float64(int16(binary.LittleEndian.Uint16(r.tmp[i:]))) / math.MaxInt16

		wet := (r.combs[0].process(dry) +
			r.combs[1].process(dry) +
			r.combs[2].process(dry) +
			r.combs[3].process(dry)) * 0.25
		wet = r.ap1.process(wet)
		wet = r.ap2.process(wet)

		out := dry*(1-r.Wet) + wet*r.Wet
		if out > 1 {
			out = 1
		}
		if out < -1 {
			out = -1
		}

		v := int16(out * math.MaxInt16)
		binary.LittleEndian.PutUint16(buf[i:], uint16(v))
		binary.LittleEndian.PutUint16(buf[i+2:], uint16(v))
	}

	return n, err
}
