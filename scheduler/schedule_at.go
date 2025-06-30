package scheduler

import (
	"strings"
	"time"
)

type ScheduleAt struct {
	At   []Time
	Mask DateMask
}

func (s *ScheduleAt) WithDateMask(m DateMask) Schedule {
	s.Mask = m
	return s
}

func (s *ScheduleAt) PlanForDate(ref time.Time) []time.Time {
	var res []time.Time

	if s.Mask.MatchTime(ref) {
		for _, t := range s.At {
			res = append(res, t.TimeForDate(ref))
		}
	}

	return res
}

func (s *ScheduleAt) String() string {
	var b strings.Builder

	b.WriteString("at ")
	for i, t := range s.At {
		if i != 0 {
			b.WriteString(", ")
		}
		b.WriteString(t.String())
	}
	b.WriteByte(' ')
	b.WriteString(s.Mask.String2())
	return b.String()
}
