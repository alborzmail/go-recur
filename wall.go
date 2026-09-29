package recur

import (
	"time"

	"github.com/alborzmail/go-recur/rscale"
)

// at is the instant a wall clock in loc reads second sec of day d. A
// time a DST gap skips takes the offset before the gap, and a time that
// occurs twice its first instant (RFC 5545 3.3.5); time.Date promises
// neither.
func at(d rscale.Day, sec int, loc *time.Location) time.Time {
	wall := int64(d)*86400 + int64(sec)
	guess := time.Unix(wall-offset(wall, loc), 0)
	o := offset(guess.Unix(), loc)
	start, end := guess.In(loc).ZoneBounds()
	// The zone periods either side of the guess, as a transition and the offsets before and after it.
	type change struct{ at, before, after int64 }
	var changes []change
	if !start.IsZero() {
		changes = append(changes, change{start.Unix(), offset(start.Unix()-1, loc), o})
	}
	if !end.IsZero() {
		changes = append(changes, change{end.Unix(), o, offset(end.Unix(), loc)})
	}
	for _, c := range changes {
		if wall >= c.at+c.before && wall < c.at+c.after {
			return time.Unix(wall-c.before, 0).In(loc)
		}
	}
	offsets := []int64{o}
	for _, c := range changes {
		offsets = append(offsets, c.before, c.after)
	}
	first, found := wall-o, false
	for _, off := range offsets {
		if x := wall - off; offset(x, loc) == off && (!found || x < first) {
			first, found = x, true
		}
	}
	return time.Unix(first, 0).In(loc)
}

// in is the instant v names when read in loc.
func (v Value) in(loc *time.Location) time.Time {
	if v.Kind == DateTime {
		return v.Time.Truncate(time.Second)
	}
	return at(rscale.DayOf(v.Time), v.clock(), loc)
}

// clock is the second of the day v's wall clock reads; a date has none.
func (v Value) clock() int {
	if v.Kind == Date {
		return 0
	}
	h, m, s := v.Time.Clock()
	return h*3600 + m*60 + s
}

func offset(unix int64, loc *time.Location) int64 {
	_, o := time.Unix(unix, 0).In(loc).Zone()
	return int64(o)
}
