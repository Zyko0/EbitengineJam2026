package entity

import "github.com/Zyko0/EbitengineJam2026/core/entity/poses"

// CatchConfig describes how an entity grabs the player on contact
type CatchConfig struct {
	Radius    float64         // horizontal proximity (blocks) that triggers the grab; <=0 disables catching
	Duration  float64         // seconds the grab/force-look lasts before the blow lands
	LookPitch float64         // forced camera pitch (radians): + looks up at a tall foe, - down at a short one
	Knockback float64         // horizontal shove speed handed to the player when the blow lands
	HitAnim   poses.Animation // strike the entity plays while grabbing
}

// Catchable reports whether this config can grab the player.
func (c CatchConfig) Catchable() bool {
	return c.Radius > 0
}

// CatchConfig returns the entity's grab parameters (zero/disabled by default).
func (b *baseEntity) CatchConfig() CatchConfig {
	return b.profile.Catch
}

// SetCaught freezes the entity onto its strike animation for the duration of a
// grab and restores its prior animation on release. Concrete entities also halt
// their own movement while Caught reports true.
func (b *baseEntity) SetCaught(caught bool) {
	if caught == b.caught {
		return
	}

	b.caught = caught
	if caught {
		b.savedAnim = b.anim
		if len(b.profile.Catch.HitAnim.Clip) > 0 {
			b.anim = b.profile.Catch.HitAnim
		}
		b.animPhase = 0 // start the strike from its first frame
	} else {
		b.anim = b.savedAnim
	}
}

// Caught reports whether the entity is mid-grab.
func (b *baseEntity) Caught() bool {
	return b.caught
}
