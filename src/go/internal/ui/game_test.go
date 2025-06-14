package ui

import "testing"

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
