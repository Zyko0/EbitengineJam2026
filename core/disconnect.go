package core

import (
	"log"

	xaudio "github.com/Zyko0/EbitengineJam2026/audio"
	"github.com/Zyko0/EbitengineJam2026/logic"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

const (
	disconnectSpeed = 0.016
	reconnectSpeed  = 0.012

	// Charge gates how long you can stay disconnected so blackout stays an
	// evasive burst, never a way to cross the maze blind. A full charge lasts
	// chargeFullSeconds of blackout and refills over chargeRefillSeconds while
	// connected. Empty charge forces a reconnect; chargeMinStart blocks
	// re-disconnecting on a near-empty meter (no micro-blackouts).
	chargeFullSeconds    = 6.0
	chargeRefillSeconds  = chargeFullSeconds
	chargeMinStartSecond = 2.0
	chargeMinStart       = chargeMinStartSecond / chargeFullSeconds

	chargeDrainPerTick  = 1.0 / (chargeFullSeconds * logic.TPS)
	chargeRefillPerTick = 1.0 / (chargeRefillSeconds * logic.TPS)
)

type DisconnectState struct {
	active bool
	locked bool // exit sequence took over: ignore the toggle key, stay connected
	T      float32
	Dir    float32 // 1 = disconnecting, 0 = reconnecting
	Charge float32 // 0..1, drains while disconnected, refills while connected
	audio  *xaudio.Disconnect
}

func NewDisconnectState() *DisconnectState {
	gen := xaudio.NewDisconnect()
	p, err := xaudio.NewPlayer(gen)
	if err != nil {
		log.Printf("disconnect audio: %v", err)
		return &DisconnectState{Charge: 1}
	}
	p.Play()

	return &DisconnectState{Charge: 1, audio: gen}
}

func (d *DisconnectState) setActive(v bool) {
	if d.active == v {
		return
	}
	d.active = v
	if d.audio != nil {
		if v {
			d.audio.ClingArmed = true
		} else {
			d.audio.ReclingArmed = true
		}
	}
}

// ForceReconnect ends any active disconnect and locks out further toggling, so
// the exit sequence can take over with the world fully connected.
func (d *DisconnectState) ForceReconnect() {
	d.setActive(false)
	d.locked = true
}

// Unlock releases the toggle lock set by ForceReconnect, re-enabling the
// disconnect key after the exit sequence hands back control on a new level.
func (d *DisconnectState) Unlock() {
	d.locked = false
}

func (d *DisconnectState) Update() {
	if !d.locked && inpututil.IsKeyJustPressed(ebiten.KeyF) {
		if d.active {
			d.setActive(false)
		} else if d.Charge >= chargeMinStart {
			d.setActive(true)
		}
	}

	// Drain while disconnected (forced reconnect when empty), refill while connected.
	if d.active {
		d.Charge = max(d.Charge-chargeDrainPerTick, 0)
		if d.Charge == 0 {
			d.setActive(false)
		}
	} else {
		d.Charge = min(d.Charge+chargeRefillPerTick, 1)
	}

	if d.active {
		d.Dir = 1
		d.T = min(d.T+disconnectSpeed, 1)
	} else {
		d.Dir = 0
		d.T = max(d.T-reconnectSpeed, 0)
	}
	if d.audio != nil {
		d.audio.T = float64(d.T)
		d.audio.Dir = float64(d.Dir)
	}
}

func (d *DisconnectState) Active() bool {
	return d.active
}
