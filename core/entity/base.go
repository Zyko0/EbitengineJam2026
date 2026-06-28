package entity

import (
	"github.com/Zyko0/EbitengineJam2026/core/entity/poses"
	"github.com/Zyko0/EbitengineJam2026/graphics"
	"github.com/Zyko0/EbitengineJam2026/logic"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/hajimehoshi/ebiten/v2"
)

type baseEntity struct {
	pos       mgl64.Vec3
	playerPos mgl64.Vec3
	profile   Profile
	anim      poses.Animation
	animPhase float64
	speed     float64
	animSpeed float64
	time      float64

	caught    bool            // grabbing the player: frozen onto the strike anim
	savedAnim poses.Animation // walk/idle anim to restore once the grab releases
}

func newBaseEntity(pos mgl64.Vec3, p Profile) *baseEntity {
	return &baseEntity{
		pos:       pos,
		profile:   p,
		speed:     1,
		animSpeed: p.AnimSpeed,
	}
}

func (b *baseEntity) update(ctx *Context) {
	const dt = 1.0 / logic.TPS
	b.playerPos = ctx.PlayerPos
	b.time += dt
	b.animPhase += b.speed * b.anim.BaseSpeed * b.animSpeed * dt
}

func (b *baseEntity) Position() mgl64.Vec3 {
	return b.pos
}

func (b *baseEntity) Profile() *Profile {
	return &b.profile
}

func (b *baseEntity) SetAnim(a poses.Animation) {
	b.anim = a
}

// boneSpec is a capsule between two joints, tapering from radius ra at a to rb
// at b. Profile.LimbRadius scales both radii per archetype.
type boneSpec struct {
	a, b   poses.Joint
	ra, rb float64
}

// baseBones is the shared humanoid skeleton: a slim waist flaring to broad
// shoulders, thin neck, and tapering limbs.
var baseBones = []boneSpec{
	// torso: slim waist flaring out to broader shoulders
	{poses.JPelvis, poses.JSpine, 0.11, 0.17},
	// neck: thin column up to the head base
	{poses.JSpine, poses.JNeck, 0.06, 0.05},
	{poses.JNeck, poses.JHead, 0.05, 0.05},
	// legs
	{poses.JPelvis, poses.JThighL, 0.085, 0.08},
	{poses.JThighL, poses.JShinL, 0.08, 0.06},
	{poses.JShinL, poses.JFootL, 0.06, 0.05},
	{poses.JPelvis, poses.JThighR, 0.085, 0.08},
	{poses.JThighR, poses.JShinR, 0.08, 0.06},
	{poses.JShinR, poses.JFootR, 0.06, 0.05},
	// arms
	{poses.JSpine, poses.JUpperArmL, 0.06, 0.05},
	{poses.JUpperArmL, poses.JForearmL, 0.05, 0.04},
	{poses.JForearmL, poses.JHandL, 0.04, 0.035},
	{poses.JSpine, poses.JUpperArmR, 0.06, 0.05},
	{poses.JUpperArmR, poses.JForearmR, 0.05, 0.04},
	{poses.JForearmR, poses.JHandR, 0.04, 0.035},
}

func (b *baseEntity) AppendGeometry(vx []ebiten.Vertex, ix []uint16, ctx *graphics.BillboardCtx) ([]ebiten.Vertex, []uint16) {
	pose := b.anim.Sample(b.animPhase)
	pose = applyProfileWarp(pose, &b.profile)
	pose = applyIdleBreath(pose, b.time)
	pose = applyLookAt(pose, b.pos, b.playerPos, ctx.Right, b.profile.LookAtCoeff)

	// Geometry construction
	ctx.PosRel = ctx.PosRel.Add(ctx.Up.Mul(b.profile.Ground))
	pelvisRel := ctx.PosRel.Add(ctx.Up.Mul(pose[poses.JPelvis][1]))
	if ctx.TF.Mul4x1(pelvisRel.Vec4(1)).W() <= 0.01 {
		return vx, ix
	}

	for _, bone := range baseBones {
		vx, ix = appendCapsule(vx, ix, ctx, pose[bone.a], pose[bone.b], bone.ra*b.profile.LimbRadius, bone.rb*b.profile.LimbRadius, 0, 0, 0)
	}
	vx, ix = appendEllipse(vx, ix, ctx, pose[poses.JHandL][0], pose[poses.JHandL][1], b.profile.HandR, b.profile.HandR, 8, 0, 0, 0)
	vx, ix = appendEllipse(vx, ix, ctx, pose[poses.JHandR][0], pose[poses.JHandR][1], b.profile.HandR, b.profile.HandR, 8, 0, 0, 0)
	hu := pose[poses.JHead][0]
	hv := pose[poses.JHead][1] + b.profile.HeadLift
	vx, ix = appendEllipse(vx, ix, ctx, hu, hv, b.profile.HeadRX, b.profile.HeadRY, 28, 0, 0, 0)

	return vx, ix
}
