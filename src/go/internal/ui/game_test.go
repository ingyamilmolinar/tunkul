package ui

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestTryAddNodeTogglesRow(t *testing.T) {
	g := New()
	g.tryAddNode(2, 0)
	if len(g.graph.Row) <= 2 || !g.graph.Row[2] {
		t.Fatalf("expected step 2 on")
	}
}

func TestDeleteNodeClearsRow(t *testing.T) {
	g := New()
	n := g.tryAddNode(1, 0)
	g.deleteNode(n)
	if len(g.graph.Row) > 1 && g.graph.Row[1] {
		t.Fatalf("expected step 1 off after delete")
	}
}

func TestAddEdgeNoDuplicates(t *testing.T) {
	g := New()
	a := g.tryAddNode(0, 0)
	b := g.tryAddNode(1, 0)
	g.addEdge(a, b)
	g.addEdge(a, b)
	if len(g.edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(g.edges))
	}
}

func TestSpawnPulseFrom(t *testing.T) {
	g := New()
	a := g.tryAddNode(0, 0)
	b := g.tryAddNode(1, 0)
	g.addEdge(a, b)
	g.spawnPulseFrom(a, 1)
	if len(g.pulses) != 1 {
		t.Fatalf("expected 1 pulse, got %d", len(g.pulses))
	}
}

func TestOnBeatUsesRoot(t *testing.T) {
	g := New()
	a := g.tryAddNode(0, 0)
	b := g.tryAddNode(2, 0)
	g.addEdge(a, b)
	g.onBeat(0)
	if len(g.pulses) != 1 {
		t.Fatalf("expected pulse from root on beat, got %d", len(g.pulses))
	}
}

func TestUpdateRunsSchedulerWhenPlaying(t *testing.T) {
	g := New()
	g.Layout(640, 480)
	// ensure first step active
	g.graph.ToggleStep(0)

	var fired int
	g.sched.OnBeat = func(int) { fired++ }
	g.drum.playing = true
	g.drum.bpm = 60

	g.Update()
	if fired == 0 {
		t.Fatalf("scheduler did not run")
	}
}

func TestClickAddsNode(t *testing.T) {
	g := New()
	g.Layout(640, 480)

	pressed := true
	restore := SetInputForTest(
		func() (int, int) { return 10, topOffset + 10 },
		func(b ebiten.MouseButton) bool { return pressed && b == ebiten.MouseButtonLeft },
		func(ebiten.Key) bool { return false },
		func() []rune { return nil },
		func() (float64, float64) { return 0, 0 },
		func() (int, int) { return 640, 480 },
	)
	defer restore()

	g.Update()
	if g.nodeAt(0, 0) == nil {
		t.Fatalf("expected node created at (0,0)")
	}
}

func TestRowLengthMatchesConnectedNodes(t *testing.T) {
	g := New()
	a := g.tryAddNode(0, 0)
	b := g.tryAddNode(1, 0)
	c := g.tryAddNode(2, 0)
	g.addEdge(a, b)
	g.addEdge(b, c)

	if len(g.graph.Row) < 3 {
		t.Fatalf("row len=%d want >=3", len(g.graph.Row))
	}
	on := 0
	for _, v := range g.graph.Row {
		if v {
			on++
		}
	}
	if on != 3 {
		t.Fatalf("active steps=%d want 3", on)
	}
	if len(g.nodes) != 3 || len(g.edges) != 2 {
		t.Fatalf("nodes=%d edges=%d", len(g.nodes), len(g.edges))
	}
}

func TestPlayStopPulseSequence(t *testing.T) {
	g := New()
	g.Layout(640, 480)
	g.graph.ToggleStep(0)
	a := g.tryAddNode(0, 0)
	b := g.tryAddNode(1, 0)
	g.addEdge(a, b)

	var beats int
	g.sched.OnBeat = func(i int) { beats++; g.onBeat(i) }

	g.drum.playing = true
	g.drum.bpm = 60
	g.Update()
	if beats == 0 || len(g.pulses) == 0 {
		t.Fatalf("expected pulse after play")
	}
	prev := len(g.pulses)

	g.drum.playing = false
	g.Update()
	if len(g.pulses) != prev {
		t.Fatalf("pulses changed after stop")
	}
}
