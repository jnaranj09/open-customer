package scheduler

import "github.com/jnaranj09/open-customer/internal/db"

// SetBeginner makes s begin its transactions on b, so a test can see the options it passes.
func SetBeginner(s *Scheduler, b db.Beginner) {
	s.beginner = b
}
