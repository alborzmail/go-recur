package recur

import (
	"fmt"
	"slices"
	"time"

	"github.com/alborzmail/go-recur/rscale"
)

// plan is a rule read against its DTSTART: its calendar, what it leaves
// out taken from DTSTART (RFC 5545 3.3.10), and its times of day.
type plan struct {
	cal       rscale.Calendar
	freq      Freq
	interval  int
	count     int
	until     *Value
	skip      Skip
	months    []rscale.Month
	weeks     []int
	yearDays  []int
	monthDays []int
	days      []WeekdayNum
	setPos    []int
	wkst      time.Weekday

	// clock is the seconds of the day the rule gives, in order; blocks
	// group them by the FREQ period they fall in, for FREQ=DAILY and
	// finer, BYSETPOS applied.
	clock  []int
	blocks []block

	start    rscale.Day
	startSec int
	// year0 is the year of the period holding DTSTART: with BYWEEKNO, the
	// year whose weeks hold it.
	year0 int
	// month0 is DTSTART's month among its year's.
	month0 int
}

type block struct {
	unit int // the period's number within its day
	secs []int
}

func newPlan(r Rule, start Value) (*plan, error) {
	if err := r.validate(); err != nil {
		return nil, err
	}
	cal, _ := r.calendar()
	day := rscale.DayOf(start.Time)
	date, ok := rscale.DateOf(cal, day)
	if !ok {
		return nil, fmt.Errorf("%w: DTSTART %s is outside the years %s reckons", ErrScale, start.Time.Format(time.DateOnly), cal.Name())
	}
	if start.Kind == Date && (r.Freq < Daily || r.ByHour != nil || r.ByMinute != nil || r.BySecond != nil) {
		return nil, syntaxf("a time of day in a rule on a date")
	}
	p := &plan{
		cal:       cal,
		freq:      r.Freq,
		interval:  max(r.Interval, 1),
		count:     r.Count,
		until:     r.Until,
		skip:      r.Skip,
		months:    r.ByMonth,
		weeks:     r.ByWeekNo,
		yearDays:  r.ByYearDay,
		monthDays: r.ByMonthDay,
		days:      r.ByDay,
		setPos:    r.BySetPos,
		wkst:      time.Monday,
		start:     day,
		startSec:  start.clock(),
	}
	if r.WeekStart != nil {
		p.wkst = *r.WeekStart
	}

	// A week or a month named without the day in it takes DTSTART's; a
	// year named alone takes its month and day.
	switch {
	case r.Freq == Yearly && r.ByYearDay == nil && r.ByMonthDay == nil && r.ByDay == nil:
		if r.ByWeekNo != nil {
			p.days = []WeekdayNum{{Day: day.Weekday()}}
		} else {
			p.monthDays = []int{date.Day}
			if r.ByMonth == nil {
				p.months = []rscale.Month{date.Month}
			}
		}
	case r.Freq == Monthly && r.ByMonthDay == nil && r.ByDay == nil:
		p.monthDays = []int{date.Day}
	case r.Freq == Weekly && r.ByDay == nil:
		p.days = []WeekdayNum{{Day: day.Weekday()}}
	}

	p.clock = clock(r, p.startSec)
	if unit := r.Freq.seconds(); unit > 0 {
		var all []block
		for _, s := range p.clock {
			if n := len(all); n > 0 && all[n-1].unit == s/unit {
				all[n-1].secs = append(all[n-1].secs, s)
			} else {
				all = append(all, block{s / unit, []int{s}})
			}
		}
		// A period of a day or less holds the same seconds whichever day
		// it falls on, so BYSETPOS picks from it once.
		for _, b := range all {
			for _, c := range p.chosen([]rscale.Day{0}, b.secs) {
				p.blocks = append(p.blocks, block{b.unit, c.secs})
			}
		}
	}

	p.year0 = date.Year
	if p.weeks != nil {
		f, _ := p.frame(date.Year)
		switch {
		case day < weekOne(f.start, p.wkst):
			p.year0--
		case day >= weekOne(f.end, p.wkst):
			p.year0++
		}
	}
	year, _ := cal.Year(date.Year)
	for p.month0 = 0; year.Months[p.month0].Month != date.Month; p.month0++ {
	}
	return p, nil
}

// clock is every second of the day the rule's BYHOUR, BYMINUTE and
// BYSECOND give. One it leaves out is DTSTART's where FREQ is coarser,
// and every value where it is not. A day has no leap second.
func clock(r Rule, sec int) []int {
	part := func(by []int, unit Freq, n, own int) []int {
		switch {
		case by != nil:
			return slices.DeleteFunc(slices.Clone(by), func(v int) bool { return v >= n })
		case r.Freq > unit:
			return []int{own}
		}
		all := make([]int, n)
		for i := range all {
			all[i] = i
		}
		return all
	}
	var out []int
	for _, h := range part(r.ByHour, Hourly, 24, sec/3600) {
		for _, m := range part(r.ByMinute, Minutely, 60, sec/60%60) {
			for _, s := range part(r.BySecond, Secondly, 60, sec%60) {
				out = append(out, h*3600+m*60+s)
			}
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// seconds is the length of a FREQ period of a day or less; 0 for longer ones.
func (f Freq) seconds() int {
	return [...]int{0, 1, 60, 3600, 86400, 0, 0, 0}[f]
}
