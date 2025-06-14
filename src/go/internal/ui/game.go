package ui

import (
	"image"
	"image/color"
	"log"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/ingyamilmolinar/tunkul/core/beat"
	"github.com/ingyamilmolinar/tunkul/core/model"
)

const topOffset = 40 // transport bar height in px

/* ─────────────── data types ───────────────────────────────────────────── */

type uiNode struct {
	ID   model.NodeID
	I, J int     // grid indices
	X, Y float64 // cached world coords (GridStep*I, GridStep*J)
}
type uiEdge struct{ A, B *uiNode }

type dragLink struct {
	from     *uiNode
	toX, toY float64
	active   bool
}

type pulse struct {
	x1, y1, x2, y2 float64
	t, speed       float64
}

type Game struct {
	// subsystems
	cam   *Camera
	split *Splitter
	drum  *DrumView
	graph *model.Graph
	sched *beat.Scheduler

	// graph
	nodes []*uiNode
	edges []uiEdge

	// visuals
	pulses []*pulse
	frame  int

	// editor state
	sel         *uiNode
	linkDrag    dragLink
	camDragging bool
	winW, winH  int
}

/* ─────────────── constructor & layout ─────────────────────────────────── */

func New() *Game {
	g := &Game{cam: NewCamera()}
	log.Print("[game] init")
	g.graph = model.NewGraph()
	g.sched = beat.NewScheduler(g.graph)
	g.split = NewSplitter(720) // temporary; real height set in Layout
	g.drum = NewDrumView(image.Rect(0, g.split.Y, 1280, 720))
	g.drum.Rows = []*DrumRow{{Name: "H"}}
	g.drum.Rows[0].Steps = g.graph.Row
	g.sched.OnBeat = func(i int) { g.onBeat(i) }
	return g
}

func (g *Game) Layout(w, h int) (int, int) {
	g.winW, g.winH = w, h
	// update splitter + drum bounds
	g.split.Y = h / 2
	g.drum.Bounds = image.Rect(0, g.split.Y, w, h)
	log.Printf("[game] layout %dx%d", w, h)
	return w, h
}

/* ─────────────── helpers — graph ops ──────────────────────────────────── */

func (g *Game) nodeAt(i, j int) *uiNode {
	for _, n := range g.nodes {
		if n.I == i && n.J == j {
			return n
		}
	}
	return nil
}

func (g *Game) tryAddNode(i, j int) *uiNode {
	if n := g.nodeAt(i, j); n != nil {
		return n
	}
	x := float64(i * GridStep)
	y := float64(j * GridStep)
	id := g.graph.AddNode(i, j)
	n := &uiNode{ID: id, I: i, J: j, X: x, Y: y}
	g.nodes = append(g.nodes, n)
	g.drum.Rows[0].Steps = g.graph.Row
	return n
}

func (g *Game) deleteNode(n *uiNode) {
	// remove node
	for i, nn := range g.nodes {
		if nn == n {
			g.nodes = append(g.nodes[:i], g.nodes[i+1:]...)
			break
		}
	}
	// prune edges touching it
	keep := g.edges[:0]
	for _, e := range g.edges {
		if e.A != n && e.B != n {
			keep = append(keep, e)
		}
	}
	g.edges = keep
	g.graph.RemoveNode(n.ID)
	g.drum.Rows[0].Steps = g.graph.Row
}

func (g *Game) addEdge(a, b *uiNode) {
	// 1. only horizontal OR vertical
	if !(a.I == b.I || a.J == b.J) {
		return // diagonals ignored
	}
	// 2. block duplicates
	for _, e := range g.edges {
		if (e.A == a && e.B == b) || (e.A == b && e.B == a) {
			return
		}
	}
	g.edges = append(g.edges, uiEdge{a, b})
}

func (g *Game) deleteEdge(a, b *uiNode) {
	for i := 0; i < len(g.edges); {
		e := g.edges[i]
		if (e.A == a && e.B == b) || (e.A == b && e.B == a) {
			g.edges[i] = g.edges[len(g.edges)-1]
			g.edges = g.edges[:len(g.edges)-1]
		} else {
			i++
		}
	}
}

/* ─────────────── input handling ───────────────────────────────────────── */

func (g *Game) handleEditor() {
	left := isMouseButtonPressed(ebiten.MouseButtonLeft)
	right := isMouseButtonPressed(ebiten.MouseButtonRight)
	shift := isKeyPressed(ebiten.KeyShiftLeft) || isKeyPressed(ebiten.KeyShiftRight)

	// coords -> world
	x, y := cursorPosition()
	wx := (float64(x) - g.cam.OffsetX) / g.cam.Scale
	wy := (float64(y-topOffset) - g.cam.OffsetY) / g.cam.Scale
	if wy > float64(g.split.Y-topOffset) { // ignore bottom pane
		return
	}
	gx, gy, i, j := Snap(wx, wy)

	// ---------------- delete node (right-click) ----------------
	if right && !shift && !left {
		if n := g.nodeAt(i, j); n != nil {
			log.Printf("[game] delete node %d,%d", i, j)
			g.deleteNode(n)
		}
		return
	}

	// ---------------- link drag (shift held OR drag in progress) ----
	if g.linkDrag.active || shift {
		g.handleLinkDrag(left, right, gx, gy, i, j)
		return
	}

	// ---------------- plain left-click → add/select node ---------
	if left && !shift && !right && !g.linkDrag.active && !g.camDragging {
		n := g.tryAddNode(i, j)
		log.Printf("[game] add/select node %d,%d", i, j)
		g.sel = n
	}
}

func (g *Game) handleLinkDrag(left, right bool, gx, gy float64, i, j int) {
	shift := isKeyPressed(ebiten.KeyShiftLeft) ||
		isKeyPressed(ebiten.KeyShiftRight)

		// start drag
	if left && !g.linkDrag.active && shift {
		if n := g.nodeAt(i, j); n != nil {
			log.Printf("[game] start link drag from %d,%d", n.I, n.J)
			g.linkDrag = dragLink{from: n, active: true}
		}
	}
	// update preview
	if g.linkDrag.active && left {
		g.linkDrag.toX, g.linkDrag.toY = gx, gy
		return
	}
	// release → commit or delete
	if g.linkDrag.active && !left {
		if n2 := g.nodeAt(i, j); n2 != nil && n2 != g.linkDrag.from {
			if right {
				log.Printf("[game] delete edge %d,%d -> %d,%d", g.linkDrag.from.I, g.linkDrag.from.J, n2.I, n2.J)
				g.deleteEdge(g.linkDrag.from, n2)
			} else {
				log.Printf("[game] add edge %d,%d -> %d,%d", g.linkDrag.from.I, g.linkDrag.from.J, n2.I, n2.J)
				g.addEdge(g.linkDrag.from, n2)
			}
		}
		log.Print("[game] end link drag")
		g.linkDrag = dragLink{}
	}
}

/* ─────────────── Update & tick ────────────────────────────────────────── */

func (g *Game) Update() error {
	// splitter
	g.split.Update()
	g.drum.Bounds.Min.Y = g.split.Y
	g.drum.Bounds.Max.Y = g.winH
	g.drum.recalcButtons()

	// camera pan only when not dragging link or splitter
	shift := isKeyPressed(ebiten.KeyShiftLeft) || isKeyPressed(ebiten.KeyShiftRight)
	panOK := !g.linkDrag.active && !g.split.dragging && !shift
	g.camDragging = g.cam.HandleMouse(panOK)

	// editor interactions
	g.handleEditor()
	g.frame++

	for i := 0; i < len(g.pulses); {
		p := g.pulses[i]
		p.t += p.speed
		if p.t >= 1 {
			g.pulses[i] = g.pulses[len(g.pulses)-1]
			g.pulses = g.pulses[:len(g.pulses)-1]
			continue
		}
		i++
	}

	g.drum.Rows[0].Steps = g.graph.Row
	// drum view logic
	g.drum.Update()
	if g.drum.playing {
		g.sched.BPM = g.drum.bpm
		log.Printf("[game] tick bpm=%d", g.sched.BPM)
		g.sched.Tick()
	}
	return nil
}

/* ─────────────── Draw ─────────────────────────────────────────────────── */

func (g *Game) Draw(screen *ebiten.Image) {
	g.drawGridPane(screen) // top
	g.drawDrumPane(screen) // bottom (includes buttons)
}

func (g *Game) drawGridPane(screen *ebiten.Image) {
	// camera matrix (shift down by bar height)
	cam := g.cam.GeoM()
	cam.Translate(0, float64(topOffset))

	topH := g.split.Y - topOffset
	frame := (g.frame / 6) % len(NodeFrames)

	// grid lattice
	for x := 0; x < g.winW; x += GridStep {
		DrawLineCam(screen, float64(x), 1, float64(x), float64(topH),
			&cam, color.RGBA{40, 40, 40, 255}, 1)
	}
	for y := 0; y < topH; y += GridStep {
		DrawLineCam(screen, 0, float64(y), float64(g.winW), float64(y),
			&cam, color.RGBA{40, 40, 40, 255}, 1)
	}

	// edges
	for _, e := range g.edges {
		DrawLineCam(screen, e.A.X, e.A.Y, e.B.X, e.B.Y,
			&cam, color.White, 2)
	}

	// link preview
	if g.linkDrag.active {
		DrawLineCam(screen, g.linkDrag.from.X, g.linkDrag.from.Y,
			g.linkDrag.toX, g.linkDrag.toY,
			&cam, color.RGBA{200, 200, 200, 255}, 2)
	}

	// nodes
	for _, n := range g.nodes {
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(n.X-16, n.Y-16)
		op.GeoM.Concat(cam)
		screen.DrawImage(NodeFrames[frame], op)
	}

	// selected highlight
	if g.sel != nil {
		DrawLineCam(screen, g.sel.X-18, g.sel.Y-18, g.sel.X+18, g.sel.Y-18, &cam, color.RGBA{255, 0, 0, 255}, 2)
		DrawLineCam(screen, g.sel.X+18, g.sel.Y-18, g.sel.X+18, g.sel.Y+18, &cam, color.RGBA{255, 0, 0, 255}, 2)
		DrawLineCam(screen, g.sel.X+18, g.sel.Y+18, g.sel.X-18, g.sel.Y+18, &cam, color.RGBA{255, 0, 0, 255}, 2)
		DrawLineCam(screen, g.sel.X-18, g.sel.Y+18, g.sel.X-18, g.sel.Y-18, &cam, color.RGBA{255, 0, 0, 255}, 2)
	}

	// pulses
	for _, p := range g.pulses {
		px := p.x1 + (p.x2-p.x1)*p.t
		py := p.y1 + (p.y2-p.y1)*p.t
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(px-8, py-8)
		op.GeoM.Concat(cam)
		screen.DrawImage(SignalDot, op)
	}

	// splitter line
	var id ebiten.GeoM // identity matrix
	DrawLineCam(screen,
		0, float64(g.split.Y),
		float64(g.winW), float64(g.split.Y),
		&id, color.RGBA{90, 90, 90, 255}, 2)
}
func (g *Game) drawDrumPane(dst *ebiten.Image) {
	g.drum.Draw(dst)
}

func (g *Game) rootNode() *uiNode {
	var root *uiNode
	for _, n := range g.nodes {
		if n.J != 0 {
			continue
		}
		inbound := false
		for _, e := range g.edges {
			if e.B == n {
				inbound = true
				break
			}
		}
		if !inbound {
			if root == nil || n.I < root.I {
				root = n
			}
		}
	}
	return root
}

func (g *Game) spawnPulseFrom(n *uiNode, beats int) {
	for _, e := range g.edges {
		if e.A == n {
			d := abs(e.A.I-e.B.I) + abs(e.A.J-e.B.J)
			speed := 1 / (float64(d*beats) * 60) // 60 fps
			g.pulses = append(g.pulses, &pulse{
				x1: e.A.X, y1: e.A.Y, x2: e.B.X, y2: e.B.Y,
				speed: speed,
			})
			log.Printf("[game] spawn pulse %d,%d -> %d,%d", e.A.I, e.A.J, e.B.I, e.B.J)
		}
	}
}

func (g *Game) onBeat(step int) {
	if root := g.rootNode(); root != nil {
		g.spawnPulseFrom(root, 1)
	}
}

/* ─────────────── math helpers ─────────────────────────────────────────── */

func atan2(y, x float64) float64 { return math.Atan2(y, x) }
func hypot(a, b float64) float64 { return math.Hypot(a, b) }
func abs(i int) int {
	if i < 0 {
		return -i
	}
	return i
}
