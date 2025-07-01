package scheduler

import (
	"strings"
	"time"
)

type ScheduleEvery struct {
	Period time.Duration
	From   Time
	Until  Time
	Mask   DateMask
}

func (s *ScheduleEvery) WithDateMask(m DateMask) Schedule {
	s.Mask = m
	return s
}

func (s *ScheduleEvery) PlanForDate(ref time.Time) []time.Time {
	var res []time.Time

	if s.Mask.MatchTime(ref) {
		t := s.From.TimeForDate(ref)
		var t1 time.Time
		if s.Until.IsDefined() {
			t1 = s.Until.TimeForDate(ref)
		} else {
			t1 = Midnight.TimeForDate(ref).AddDate(0, 0, 1)
		}
		for t.Before(t1) {
			res = append(res, t)
			t = t.Add(s.Period)
		}
	}

	return res
}

func (s *ScheduleEvery) String() string {
	var b strings.Builder

	b.WriteString("every ")
	b.WriteString(s.Period.String())
	if s.From != -1 {
		b.WriteString("from ")
		b.WriteString(s.From.String())
	}
	if s.Until != -1 {
		b.WriteString("until ")
		b.WriteString(s.Until.String())
	}
	b.WriteByte(' ')
	b.WriteString(s.Mask.String2())
	return b.String()
}
