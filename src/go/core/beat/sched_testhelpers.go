//go:build test

package beat

import "time"

// SetNowFunc allows tests to override the scheduler's clock.
func (s *Scheduler) SetNowFunc(f func() time.Time) {
	s.now = f
}
