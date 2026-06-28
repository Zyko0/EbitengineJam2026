// Command animconv bakes a Collada (.dae) skeletal animation
// into a Go []Pose keyframe table for the 16-joint billboard rig in
// core/entity/poses.
//
//	go run ./cmd/animconv -in walking.dae
package main

import (
	"bytes"
	"encoding/xml"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"github.com/go-gl/mathgl/mgl64"
)

// jointTargets maps each rig joint (in Joint iota order) to candidate Mixamo
// bone names, by priority. The emitted Pose literal lists joints in this order.
var jointTargets = []struct {
	name       string
	candidates []string
}{
	{"JPelvis", []string{"Hips"}},
	{"JSpine", []string{"Spine2", "Spine1", "Spine"}},
	{"JNeck", []string{"Neck"}},
	{"JHead", []string{"Head"}},
	{"JThighL", []string{"LeftUpLeg"}},
	{"JShinL", []string{"LeftLeg"}},
	{"JFootL", []string{"LeftToeBase", "LeftFoot"}},
	{"JThighR", []string{"RightUpLeg"}},
	{"JShinR", []string{"RightLeg"}},
	{"JFootR", []string{"RightToeBase", "RightFoot"}},
	{"JUpperArmL", []string{"LeftArm"}},
	{"JForearmL", []string{"LeftForeArm"}},
	{"JHandL", []string{"LeftHand"}},
	{"JUpperArmR", []string{"RightArm"}},
	{"JForearmR", []string{"RightForeArm"}},
	{"JHandR", []string{"RightHand"}},
}

const jointCount = 16

func main() {
	in := flag.String("in", "female_walking.dae", "input Collada .dae file")
	headV := flag.Float64("head", 3, "target v height of the head joint (sets scale)")
	depth := flag.Float64("depth", 0., "weight folding fore/aft (depth) limb swing into lateral u")
	flip := flag.Bool("flip", false, "flip lateral (u) axis")
	flag.Parse()

	base := strings.TrimSuffix(filepath.Base(*in), filepath.Ext(*in))
	varName := pascal(base)
	out := filepath.Join("core/entity/poses", base+"_gen.go")

	data, err := os.ReadFile(*in)
	if err != nil {
		fatal("read %s: %v", *in, err)
	}

	doc, err := parseCollada(data)
	if err != nil {
		fatal("parse: %v", err)
	}

	frames, err := bake(doc, *headV, *depth, *flip)
	if err != nil {
		fatal("bake: %v", err)
	}

	src := codegen(frames, varName, *in)
	if err := os.WriteFile(out, src, 0o644); err != nil {
		fatal("write %s: %v", out, err)
	}
	fmt.Fprintf(os.Stderr, "wrote %d frames (%s) -> %s\n", len(frames), varName, out)
}

// pascal converts a file base name like "female_walking" into an exported Go
// identifier "FemaleWalking", splitting on common separators.
func pascal(s string) string {
	var b strings.Builder
	upNext := true
	for _, r := range s {
		switch {
		case r == '_' || r == '-' || r == ' ' || r == '.':
			upNext = true
		case upNext:
			b.WriteRune(unicode.ToUpper(r))
			upNext = false
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func fatal(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "animconv: "+format+"\n", a...)
	os.Exit(1)
}

// ---- Collada XML model ----

type collada struct {
	XMLName xml.Name `xml:"COLLADA"`
	Asset   struct {
		UpAxis string `xml:"up_axis"`
	} `xml:"asset"`
	LibAnimations struct {
		Animations []animation `xml:"animation"`
	} `xml:"library_animations"`
	LibVisualScenes struct {
		Scenes []struct {
			Nodes []node `xml:"node"`
		} `xml:"visual_scene"`
	} `xml:"library_visual_scenes"`
}

type animation struct {
	Animations []animation `xml:"animation"`
	Sources    []source    `xml:"source"`
	Samplers   []sampler   `xml:"sampler"`
	Channels   []channel   `xml:"channel"`
}

type source struct {
	ID         string `xml:"id,attr"`
	FloatArray struct {
		Data string `xml:",chardata"`
	} `xml:"float_array"`
}

type sampler struct {
	ID     string  `xml:"id,attr"`
	Inputs []input `xml:"input"`
}

type input struct {
	Semantic string `xml:"semantic,attr"`
	Source   string `xml:"source,attr"`
}

type channel struct {
	Source string `xml:"source,attr"`
	Target string `xml:"target,attr"`
}

type node struct {
	ID     string     `xml:"id,attr"`
	Name   string     `xml:"name,attr"`
	Matrix []matrixEl `xml:"matrix"`
	Nodes  []node     `xml:"node"`
}

type matrixEl struct {
	Data string `xml:",chardata"`
}

func parseCollada(data []byte) (*collada, error) {
	var doc collada
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	return &doc, nil
}

// ---- baking ----

// rtNode is a runtime skeleton node with its bind transform and optional
// per-frame animated local transforms.
type rtNode struct {
	id, bone string
	bind     mgl64.Mat4
	anim     []mgl64.Mat4
	children []*rtNode
}

func bake(doc *collada, headV, depthW float64, flip bool) ([][jointCount][2]float64, error) {
	// Flatten the animation library and resolve channels -> per-node matrices.
	srcMap := map[string][]float64{}
	sampMap := map[string]sampler{}
	var channels []channel
	var collect func(a animation)
	collect = func(a animation) {
		for _, s := range a.Sources {
			srcMap["#"+s.ID] = parseFloats(s.FloatArray.Data)
		}
		for _, s := range a.Samplers {
			sampMap["#"+s.ID] = s
		}
		channels = append(channels, a.Channels...)
		for _, c := range a.Animations {
			collect(c)
		}
	}
	for _, a := range doc.LibAnimations.Animations {
		collect(a)
	}

	animMap := map[string][]mgl64.Mat4{} // node id -> frames
	frameCount := 0
	for _, ch := range channels {
		nodeID := ch.Target
		if i := strings.IndexByte(nodeID, '/'); i >= 0 {
			nodeID = nodeID[:i]
		}
		samp, ok := sampMap[ch.Source]
		if !ok {
			continue
		}
		var outRef string
		for _, in := range samp.Inputs {
			if in.Semantic == "OUTPUT" {
				outRef = in.Source
			}
		}
		floats := srcMap[outRef]
		if len(floats) == 0 || len(floats)%16 != 0 {
			return nil, fmt.Errorf("channel %q: expected baked 4x4 matrices (got %d floats); re-export with matrix transforms", ch.Target, len(floats))
		}
		n := len(floats) / 16
		mats := make([]mgl64.Mat4, n)
		for i := range mats {
			mats[i] = mat4RowMajor(floats[i*16 : i*16+16])
		}
		animMap[nodeID] = mats
		if n > frameCount {
			frameCount = n
		}
	}
	if frameCount == 0 {
		return nil, fmt.Errorf("no animation channels found")
	}

	// Build the runtime skeleton tree.
	if len(doc.LibVisualScenes.Scenes) == 0 {
		return nil, fmt.Errorf("no visual scene")
	}
	var roots []*rtNode
	for _, n := range doc.LibVisualScenes.Scenes[0].Nodes {
		roots = append(roots, buildNode(n, animMap))
	}

	zUp := strings.EqualFold(strings.TrimSpace(doc.Asset.UpAxis), "Z_UP")

	// FK every frame -> world positions per bone -> raw (lateral,height,depth) per joint.
	raw := make([][jointCount][3]float64, frameCount)
	var missing []string
	for f := 0; f < frameCount; f++ {
		pos := map[string]mgl64.Vec3{}
		for _, r := range roots {
			fkWorld(r, mgl64.Ident4(), f, frameCount, pos)
		}
		for j, t := range jointTargets {
			p, ok := lookup(pos, t.candidates)
			if !ok {
				if f == 0 {
					missing = append(missing, t.name)
				}
				continue
			}
			raw[f][j] = project(p, zUp)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("joints not found in skeleton: %s\navailable bones: %s",
			strings.Join(missing, ", "), strings.Join(boneNames(roots), ", "))
	}

	frames := normalize(raw, headV, depthW, flip)
	frames = dropDuplicateLast(frames)

	logDiagnostics(frameCount, len(frames), frames)
	return frames, nil
}

func buildNode(n node, animMap map[string][]mgl64.Mat4) *rtNode {
	r := &rtNode{id: n.ID, bone: boneName(firstNonEmpty(n.Name, n.ID))}
	r.bind = mgl64.Ident4()
	if len(n.Matrix) > 0 {
		if fs := parseFloats(n.Matrix[0].Data); len(fs) == 16 {
			r.bind = mat4RowMajor(fs)
		}
	}
	r.anim = animMap[n.ID]
	for _, c := range n.Nodes {
		r.children = append(r.children, buildNode(c, animMap))
	}
	return r
}

func fkWorld(n *rtNode, parent mgl64.Mat4, f, frameCount int, out map[string]mgl64.Vec3) {
	local := n.bind
	if len(n.anim) > 0 {
		local = n.anim[sampleIdx(f, frameCount, len(n.anim))]
	}
	world := parent.Mul4(local)
	c := world.Col(3)
	out[n.bone] = mgl64.Vec3{c[0], c[1], c[2]}
	for _, ch := range n.children {
		fkWorld(ch, world, f, frameCount, out)
	}
}

// sampleIdx maps master frame f (of frameCount) to a joint with n samples.
func sampleIdx(f, frameCount, n int) int {
	if n == frameCount || frameCount <= 1 {
		if f < n {
			return f
		}
		return n - 1
	}
	i := int(math.Round(float64(f) * float64(n-1) / float64(frameCount-1)))
	if i >= n {
		i = n - 1
	}
	return i
}

// project returns (lateral, height, depth) for the billboard front view.
func project(p mgl64.Vec3, zUp bool) [3]float64 {
	if zUp {
		return [3]float64{p.X(), p.Z(), p.Y()}
	}
	return [3]float64{p.X(), p.Y(), p.Z()}
}

// normalize centers laterally on the pelvis, drops feet to v=0, scales so the
// head joint sits near headV, and folds the fore/aft (depth) limb swing into u
// (per-frame relative to the pelvis, which also removes root forward travel).
func normalize(raw [][jointCount][3]float64, headV, depthW float64, flip bool) [][jointCount][2]float64 {
	const (
		jPelvis = 0
		jHead   = 3
		jFootL  = 6
		jFootR  = 9
	)
	pelvisLat := 0.0
	footMinV := math.Inf(1)
	for _, fr := range raw {
		pelvisLat += fr[jPelvis][0]
		footMinV = math.Min(footMinV, math.Min(fr[jFootL][1], fr[jFootR][1]))
	}
	pelvisLat /= float64(len(raw))

	scale := 1.0
	if h := raw[0][jHead][1] - footMinV; h > 1e-6 {
		scale = headV / h
	}
	sign := 1.0
	if flip {
		sign = -1.0
	}

	out := make([][jointCount][2]float64, len(raw))
	for f, fr := range raw {
		pelvisDep := fr[jPelvis][2]
		for j := range fr {
			lat := fr[j][0] - pelvisLat
			dep := fr[j][2] - pelvisDep
			out[f][j][0] = (lat + depthW*dep) * scale * sign
			out[f][j][1] = (fr[j][1] - footMinV) * scale
		}
	}
	return out
}

func dropDuplicateLast(frames [][jointCount][2]float64) [][jointCount][2]float64 {
	if len(frames) < 2 {
		return frames
	}
	a, b := frames[0], frames[len(frames)-1]
	var d float64
	for j := range a {
		d += math.Hypot(a[j][0]-b[j][0], a[j][1]-b[j][1])
	}
	if d < 0.01 {
		return frames[:len(frames)-1]
	}
	return frames
}

// ---- helpers ----

func lookup(pos map[string]mgl64.Vec3, candidates []string) (mgl64.Vec3, bool) {
	for _, c := range candidates {
		if p, ok := pos[c]; ok {
			return p, true
		}
	}
	return mgl64.Vec3{}, false
}

func boneNames(roots []*rtNode) []string {
	var names []string
	var walk func(n *rtNode)
	walk = func(n *rtNode) {
		names = append(names, n.bone)
		for _, c := range n.children {
			walk(c)
		}
	}
	for _, r := range roots {
		walk(r)
	}
	return names
}

// boneName strips a "mixamorig" namespace prefix and separators.
func boneName(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.LastIndexAny(s, ":"); i >= 0 {
		s = s[i+1:]
	}
	s = strings.TrimPrefix(s, "mixamorig")
	s = strings.TrimLeft(s, "_:")
	return s
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

func parseFloats(s string) []float64 {
	fields := strings.Fields(s)
	out := make([]float64, 0, len(fields))
	for _, f := range fields {
		v, err := strconv.ParseFloat(f, 64)
		if err != nil {
			continue
		}
		out = append(out, v)
	}
	return out
}

// mat4RowMajor builds a column-major mgl64.Mat4 from 16 row-major Collada floats.
func mat4RowMajor(d []float64) mgl64.Mat4 {
	return mgl64.Mat4{
		d[0], d[4], d[8], d[12],
		d[1], d[5], d[9], d[13],
		d[2], d[6], d[10], d[14],
		d[3], d[7], d[11], d[15],
	}
}

func logDiagnostics(srcFrames, outFrames int, frames [][jointCount][2]float64) {
	fmt.Fprintf(os.Stderr, "frames: %d source -> %d baked\n", srcFrames, outFrames)
	if len(frames) == 0 {
		return
	}
	f0 := frames[0]
	fmt.Fprintf(os.Stderr, "frame0 pelvis=(%.2f,%.2f) head=(%.2f,%.2f) footL=(%.2f,%.2f) handL=(%.2f,%.2f)\n",
		f0[0][0], f0[0][1], f0[3][0], f0[3][1], f0[6][0], f0[6][1], f0[12][0], f0[12][1])
}

func codegen(frames [][jointCount][2]float64, varName, srcFile string) []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, "// Code generated by cmd/animconv from %s. DO NOT EDIT.\n\n", srcFile)
	b.WriteString("package poses\n\n")
	fmt.Fprintf(&b, "var %s = Clip{\n", varName)
	for _, fr := range frames {
		b.WriteString("\t{")
		for j, p := range fr {
			if j > 0 {
				b.WriteString(", ")
			}
			fmt.Fprintf(&b, "{%.4f, %.4f}", p[0], p[1])
		}
		b.WriteString("},\n")
	}
	b.WriteString("}\n")
	return b.Bytes()
}
