package scheduler

import (
	"fmt"
	"strings"
	"time"
)

type Time int // seconds from midnight

var UndefinedTime = Time(-1)
var Midnight = Time(0)

func TimeFromHourMinSec(h, m, s int) Time {
	return Time(h*3600 + m*60 + s)
}

func (t Time) IsDefined() bool {
	return t != UndefinedTime
}

func (t Time) HourMinSec() (int, int, int) {
	h, r := int(t)/3600, int(t)%3600
	m, s := r/60, r%60
	return h, m, s
}

func (t Time) Duration() time.Duration {
	return time.Duration(t) * time.Second
}

func (t Time) TimeForDate(d time.Time) time.Time {
	if !t.IsDefined() {
		return Midnight.TimeForDate(d)
	}
	year, month, day := d.Date()
	hour, min, sec := t.HourMinSec()
	return time.Date(year, month, day, hour, min, sec, 0, d.Location())
}

func (t Time) String() string {
	if !t.IsDefined() {
		return "-"
	}
	h, m, s := t.HourMinSec()
	if s == 0 {
		return fmt.Sprintf("%02d:%02d", h, m)
	}
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}

type DateMask uint64

const DayShift int = 0
const MonthShift int = 31
const WeekdayShift int = 43

const DayMask DateMask = (1<<31 - 1) << DayShift
const MonthMask DateMask = (1<<12 - 1) << MonthShift
const WeekdayMask DateMask = (1<<7 - 1) << WeekdayShift

const EveryDay = DayMask | MonthMask | WeekdayMask

func MakeWeekday(d time.Weekday) DateMask {
	return DateMask(1 << d << WeekdayShift)
}

func (d DateMask) String() string {
	var s strings.Builder

	s.WriteString("[")
	pfx := ""
	for i := range 7 {
		w := time.Weekday(i)
		if d.MatchWeekday(w) {
			s.WriteString(pfx)
			s.WriteString(strings.ToLower(w.String()[:3]))
			pfx = ","
		}
	}
	s.WriteString("]")

	s.WriteString("[")
	pfx = ""
	for i := range 12 {
		m := time.Month(i + 1)
		if d.MatchMonth(m) {
			s.WriteString(pfx)
			s.WriteString(strings.ToLower(m.String()[:3]))
			pfx = ","
		}
	}
	s.WriteString("]")

	s.WriteString("[")
	pfx = ""
	for i := range 31 {
		day := i + 1
		if d.MatchDay(day) {
			s.WriteString(pfx)
			s.WriteString(fmt.Sprintf("%v", day))
			pfx = ","
		}
	}
	s.WriteString("]")

	return s.String()
}

func (d DateMask) String2() string {
	var s strings.Builder

	if d.Match(WeekdayMask) {
		return ""
	}

	pfx := "on "
	for i := range 7 {
		w := time.Weekday(i)
		if d.MatchWeekday(w) {
			s.WriteString(pfx)
			s.WriteString(strings.ToLower(w.String()))
			pfx = ", "
		}
	}

	return s.String()
}

func (d DateMask) ClearDayMask() DateMask {
	return d &^ DayMask
}

func (d DateMask) SetDayMask(m DateMask) DateMask {
	return d.ClearDayMask() | (m & DayMask)
}

func (d DateMask) ClearMonthMask() DateMask {
	return d &^ MonthMask
}

func (d DateMask) SetMonthMask(m DateMask) DateMask {
	return d.ClearMonthMask() | (m & MonthMask)
}

func (d DateMask) ClearWeekdayMask() DateMask {
	return d &^ WeekdayMask
}

func (d DateMask) SetWeekdayMask(m DateMask) DateMask {
	return d.ClearWeekdayMask() | (m & WeekdayMask)
}

func (d DateMask) Match(m DateMask) bool {
	return m&d == m
}

func (d DateMask) MatchWeekday(w time.Weekday) bool {
	return d.Match(1 << w << WeekdayShift)
}

func (d DateMask) MatchDay(day int) bool {
	return d.Match(1 << (day - 1) << DayShift)
}

func (d DateMask) MatchMonth(month time.Month) bool {
	return d.Match(1 << (month - 1) << MonthShift)
}

func (d DateMask) MatchTime(t time.Time) bool {
	return d.Match(timeAsDateMask(t))
}

func timeAsDateMask(t time.Time) DateMask {
	_, month, day := t.Date()
	return 1<<t.Weekday()<<WeekdayShift |
		1<<(day-1)<<DayShift |
		1<<(month-1)<<MonthShift
}
