package entity

import (
	"math"

	"github.com/Zyko0/EbitengineJam2026/core/entity/poses"
	"github.com/go-gl/mathgl/mgl64"
)

// applyIdleBreath adds a slow breathing bob to the spine/head
func applyIdleBreath(p poses.Pose, t float64) poses.Pose {
	const blend = 1.75
	const bobScale = 1.25

	bob := math.Sin(t*1.2) * 0.03 * blend * bobScale
	shoulder := math.Sin(t*1.2+0.3) * 0.015 * blend * bobScale
	p[poses.JSpine][1] += bob
	p[poses.JNeck][1] += bob
	p[poses.JHead][1] += bob * 0.5
	p[poses.JUpperArmL][1] += shoulder
	p[poses.JUpperArmR][1] += shoulder

	return p
}

// applyLookAt shifts neck/head in billboard u toward the player's lateral position.
func applyLookAt(p poses.Pose, entityPos, playerPos, camRight mgl64.Vec3, coeff float64) poses.Pose {
	dx := playerPos.X() - entityPos.X()
	dz := playerPos.Z() - entityPos.Z()
	l := math.Sqrt(dx*dx + dz*dz)
	if l < 1e-6 {
		return p
	}
	// Project entity-to-player onto camera right to get billboard-space lateral.
	lateral := (dx*camRight.X() + dz*camRight.Z()) / l
	if lateral > 1 {
		lateral = 1
	} else if lateral < -1 {
		lateral = -1
	}
	lateral *= 1.5 * coeff
	p[poses.JSpine][0] += 0.04 * lateral
	p[poses.JNeck][0] += 0.10 * lateral
	p[poses.JHead][0] += 0.18 * lateral

	return p
}
