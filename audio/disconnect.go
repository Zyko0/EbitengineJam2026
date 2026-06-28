package audio

import (
	"encoding/binary"
	"math"
	"math/rand"
)

// Disconnect generates electrical "bzz" bursts synced to the disconnect animation flicker
type Disconnect struct {
	t          float64
	rng        *rand.Rand
	crackleAmp float64
	crackleT   float64
	snapT      float64
	snapType   int // 0=none, 1=disconnect zap, 2=reconnect click

	T            float64
	Dir          float64
	ClingArmed   bool
	ReclingArmed bool
}

func NewDisconnect() *Disconnect {
	return &Disconnect{rng: rand.New(rand.NewSource(54321))}
}

func dcSmoothstep(e0, e1, x float64) float64 {
	t := (x - e0) / (e1 - e0)
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	return t * t * (3 - 2*t)
}

func dcHash(n float64) float64 {
	x := math.Sin(n*127.1+311.7) * 43758.5453
	return x - math.Floor(x)
}

func (d *Disconnect) Read(buf []byte) (int, error) {
	const dt = 1.0 / sampleRate
	n := len(buf) / 4

	if d.ClingArmed {
		d.snapT = 0
		d.snapType = 1
		d.ClingArmed = false
	}
	if d.ReclingArmed {
		d.snapT = 0
		d.snapType = 2
		d.ReclingArmed = false
	}

	for i := 0; i < n; i++ {
		d.t += dt
		d.snapT += dt
		d.crackleT += dt

		// Flicker envelope matching the shader's disconnect/reconnect flicker windows.
		var flickerEnv float64
		if d.Dir > 0.5 {
			flickerEnv = dcSmoothstep(0.02, 0.2, d.T) * dcSmoothstep(0.75, 0.3, d.T)
		} else {
			flickerEnv = dcSmoothstep(0.85, 0.45, d.T) * dcSmoothstep(0.35, 0.55, d.T)
		}
		// Random modulation at 6Hz + fine 14Hz, matching shader's flickerRng.
		flickerRng := dcHash(math.Floor(d.t*6)*17.3 + 0.42)
		flickerFine := dcHash(math.Floor(d.t*14)*8.7+0.57) * 0.15
		flickerMod := math.Min(flickerRng*0.85+flickerFine, 1.0)
		flickerEnv *= 0.4 + 0.6*flickerMod

		// Electrical buzz: 60Hz hum + harmonics + slow flicker.
		hum := math.Sin(2*math.Pi*240*d.t)*0.3 +
			math.Sin(2*math.Pi*180*d.t)*0.3 +
			math.Sin(2*math.Pi*360*d.t)*0.4
		hum *= 0.8 // + 0*0.3*math.Sin(2*math.Pi*4*d.t)

		// Random crackle bursts.
		if d.crackleT > 0.008+d.rng.Float64()*0.04 {
			d.crackleT = 0
			d.crackleAmp = 0.8 + d.rng.Float64()*0.8
		}
		crackle := (d.rng.Float64()*2 - 1) * d.crackleAmp
		d.crackleAmp *= 0.9992

		buzz := (hum*0.35 + crackle*0.2) * flickerEnv * 0.75

		sample := buzz
		if sample > 1 {
			sample = 1
		}
		if sample < -1 {
			sample = -1
		}

		v := int16(sample * math.MaxInt16)
		off := i * 4
		binary.LittleEndian.PutUint16(buf[off:], uint16(v))
		binary.LittleEndian.PutUint16(buf[off+2:], uint16(v))
	}

	return n * 4, nil
}
