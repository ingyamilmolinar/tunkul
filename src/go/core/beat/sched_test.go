package beat

import (
	"testing"
	"time"

	"github.com/ingyamilmolinar/tunkul/core/model"
)

func TestSchedulerFiresEveryBeat(t *testing.T) {
	m := model.NewGraph()
	m.ToggleStep(0)

	now := time.Now()
	fake := func() time.Time { now = now.Add(time.Second); return now }

	fired := 0
	s := NewScheduler(m)
	s.BPM = 60
	s.now = fake
	s.OnBeat = func(_ int) { fired++ }

	for i := 0; i < 5; i++ {
		s.Tick()
	}
	if fired != 5 {
		t.Fatalf("expected 5 beats, got %d", fired)
	}
}
