package entity

import "github.com/Zyko0/EbitengineJam2026/core/entity/poses"

// Profile is a per-enemy visual identity
type Profile struct {
	Name string

	// Motion identity.
	Anim      poses.Animation
	AnimSpeed float64 // base walk-cycle tempo (1 = baked)
	BobScale  float64 // idle-breath amplitude (1 = default, 0 = perfectly still)

	// Skeleton warp, applied to the baked pose before drawing.
	HeightScale float64 // vertical stretch about the floor
	WidthScale  float64 // horizontal stretch about the centre line (stance + shoulders)
	Hunch       float64 // curls the upper spine downward (0 upright)

	// Build.
	LimbRadius float64 // scales every bone capsule radius

	// Head ellipse.
	HeadRX, HeadRY, HeadLift float64

	HandR  float64 // hand blob radius
	Ground float64 // lift so the feet rest on the floor instead of in it

	LookAtCoeff float64 // the scale the neck can go to to follo player's camera tilt

	// How this archetype grabs the player on contact (height/strike vary per foe).
	Catch CatchConfig
}

// Humanoid is the baseline shape: the hand-tuned hunter proportions every other
// archetype derives from. Kept verbatim so the "nice humanoid" silhouette stays
// available as its own model.
var Humanoid = Profile{
	Name:        "Humanoid",
	Anim:        poses.StabbingAnim,
	AnimSpeed:   1,
	BobScale:    1,
	HeightScale: 1,
	WidthScale:  1,
	Hunch:       0,
	LimbRadius:  1,
	HeadRX:      0.195,
	HeadRY:      0.31,
	HeadLift:    0.05,
	HandR:       0.06,
	Ground:      0.08,
	LookAtCoeff: 1,
	// Eye-level grab: player and humanoid share a height, so the gaze stays
	// roughly flat. variant() copies this onto every archetype unless overridden.
	Catch: CatchConfig{
		Radius:    2,
		Duration:  0.7,
		LookPitch: 0.5,
		Knockback: 0.75,
		HitAnim:   poses.StabbingAnim,
	},
}

// variant clones base, renames it, drops inherited features and applies overrides.
func variant(base Profile, name string, f func(*Profile)) Profile {
	p := base
	p.Name = name
	f(&p)

	return p
}

// The distinct enemy archetypes. Each combines a few levers (scale, build, head,
// gait, features) so it reads as a different creature from the same rig.
var (
	// TallOne: towering and gaunt, small head, near-still glide.
	TallOne = variant(Humanoid, "TallOne", func(p *Profile) {
		p.HeightScale = 1.5
		p.WidthScale = 1.
		p.LimbRadius = 0.6
		p.HeadRX, p.HeadRY = 0.14, 0.3
		p.Anim = poses.DrunkWalkAnim // and poses.ZombieRunAnim // At random
		p.BobScale = 0
		// Looms over the player: the grab cranes the gaze upward, lingers, and a
		// downward slash flings the player hard.
		p.Catch = CatchConfig{
			Radius:    2.5,
			Duration:  1.,
			LookPitch: 0.65,
			Knockback: 1.5,
			HitAnim:   poses.GreatSwordSlashAnim,
		}
	})

	// Spider: low, splayed limbs; the ceiling stalker's look (see SpiderStalker).
	Spider = variant(Humanoid, "Spider", func(p *Profile) {
		p.LimbRadius = 0.9
		p.HeadRX, p.HeadRY = 0.17, 0.24
		p.Anim = poses.LowCrawlAnim
		p.BobScale = 0.
		p.HeightScale = 1.25
		p.WidthScale = 0.75
		p.LookAtCoeff = 2.
		// Reaches the player on the ground and hits them; no dedicated strike, it
		// just keeps crawling (HitAnim left zero so SetCaught holds the walk). The
		// grab still cranes the view down onto the low crawler.
		p.Catch = CatchConfig{
			Radius:    1.6,
			Duration:  0.5,
			LookPitch: 0.9,
			Knockback: 0.5,
		}
	})

	// Small: child-sized, quick and jittery.
	Small = variant(Humanoid, "Small", func(p *Profile) {
		p.HeightScale = 0.55
		p.WidthScale = 0.9
		p.HeadRX, p.HeadRY = 0.16, 0.24
		p.Anim = poses.RunningAnim
		// Low to the ground: the gaze is dragged down and the shove is light.
		p.Catch = CatchConfig{
			Radius:    1.2,
			Duration:  0.5,
			LookPitch: -0.5,
			Knockback: 0.12,
			HitAnim:   poses.KickingAnim,
		}
	})
)

// applyProfileWarp stretches and stoops the baked pose per the profile
func applyProfileWarp(p poses.Pose, pr *Profile) poses.Pose {
	for i := range p {
		p[i][0] *= pr.WidthScale
		p[i][1] *= pr.HeightScale
	}
	if pr.Hunch != 0 {
		h := pr.Hunch
		p[poses.JSpine][1] -= 0.2 * h
		p[poses.JNeck][1] -= 0.5 * h
		p[poses.JHead][1] -= 0.8 * h
		p[poses.JUpperArmL][1] -= 0.2 * h
		p[poses.JUpperArmR][1] -= 0.2 * h
	}

	return p
}
