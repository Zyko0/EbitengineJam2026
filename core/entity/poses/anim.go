package poses

// Animation pairs a baked Clip (generated data) with a hand-tuned base tempo.
// BaseSpeed scales playback independently of an entity's movement/anim speed
type Animation struct {
	Clip      Clip
	BaseSpeed float64
}

// Sample returns the pose at the given phase, delegating to the underlying clip.
func (a Animation) Sample(phase float64) Pose {
	return a.Clip.Sample(phase)
}

// Hand-tuned animations: adjust BaseSpeed for any clip that plays too fast/slow.
var (
	StabbingAnim        = Animation{Clip: Stabbing, BaseSpeed: 0.5}
	DrunkWalkAnim       = Animation{Clip: DrunkWalk, BaseSpeed: 0.4}
	LowCrawlAnim        = Animation{Clip: LowCrawl, BaseSpeed: 1}
	RunningAnim         = Animation{Clip: Running, BaseSpeed: 0.8}
	WalkingAnim         = Animation{Clip: Walking, BaseSpeed: 1}
	ZombieRunAnim       = Animation{Clip: ZombieRun, BaseSpeed: 0.9}
	CatwalkWalkingAnim  = Animation{Clip: CatwalkWalking, BaseSpeed: 0.9}
	KickingAnim         = Animation{Clip: Kicking, BaseSpeed: 0.6}
	JazzDancingAnim     = Animation{Clip: JazzDancing, BaseSpeed: 0.7}
	GreatSwordSlashAnim = Animation{Clip: GreatSwordSlash, BaseSpeed: 0.7}
)

