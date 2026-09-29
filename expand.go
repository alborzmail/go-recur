package recur

import (
	"iter"
	"slices"
	"time"

	"github.com/alborzmail/go-recur/rscale"
)

// reach is how many days before its period SKIP=BACKWARD may move a
// day: a week before week 1, which starts up to three days before its
// year. The same bounds the days after it.
const reach = 10

// dayClock is a day and the seconds of it an instance falls on.
type dayClock struct {
	day  rscale.Day
	secs []int
}

type month struct {
	year  int
	id    rscale.Month
	start rscale.Day
	days  int
}

func (m month) end() rscale.Day { return m.start + rscale.Day(m.days) }

// frame is a year's months with those of the years either side, so that
// a gap at the year's ends resolves and a day moved over them is placed.
type frame struct {
	year       int
	start, end rscale.Day
	months     []month // consecutive; the years either side only where the calendar reckons them
}

func (p *plan) frame(y int) (frame, bool) {
	year, ok := p.cal.Year(y)
	if !ok {
		return frame{}, false
	}
	f := frame{year: y, start: year.Start, end: year.End()}
	for _, y := range []int{y - 1, y, y + 1} {
		year, ok := p.cal.Year(y)
		if !ok {
			continue
		}
		d := year.Start
		for _, s := range year.Months {
			f.months = append(f.months, month{y, s.Month, d, s.Days})
			d += rscale.Day(s.Days)
		}
	}
	return f, true
}

// own is the index range of year y's months among the frame's.
func (f frame) own(y int) (int, int) {
	lo := slices.IndexFunc(f.months, func(m month) bool { return m.year == y })
	hi := lo
	for hi < len(f.months) && f.months[hi].year == y {
		hi++
	}
	return lo, hi
}

func (f frame) locate(d rscale.Day) (int, bool) {
	for i, m := range f.months {
		if d >= m.start && d < m.end() {
			return i, true
		}
	}
	return 0, false
}

// gap picks the member before a missing one or the one after it.
func (s Skip) gap(before, after int) (int, bool) {
	switch s {
	case Backward:
		return before, true
	case Forward:
		return after, true
	}
	return 0, false
}

// place is the index of the nth of count members, counted from the end
// when n is negative. Past either end is a gap: -1 or count when SKIP
// moves over it.
func (s Skip) place(n, count int) (int, bool) {
	switch {
	case n > 0 && n <= count:
		return n - 1, true
	case n < 0 && -n <= count:
		return count + n, true
	case n > 0:
		return s.gap(count-1, count)
	}
	return s.gap(-1, 0)
}

func beyond(n, count int) bool { return n > count || -n > count }

// monthsOf is the months of year y BYMONTH names, SKIP applied; all of
// them without BYMONTH.
func (p *plan) monthsOf(f frame, y int) []month {
	lo, hi := f.own(y)
	own := f.months[lo:hi]
	if p.months == nil {
		return own
	}
	var out []month
	for _, want := range p.months {
		pos := 0
		for pos < len(own) && own[pos].id.Before(want) {
			pos++
		}
		if pos < len(own) && own[pos].id == want {
			out = append(out, own[pos])
		} else if i, ok := p.skip.gap(pos-1, pos); ok && lo+i >= 0 && lo+i < len(f.months) {
			out = append(out, f.months[lo+i])
		}
	}
	slices.SortFunc(out, func(a, b month) int { return int(a.start - b.start) })
	return slices.Compact(out)
}

// pick is the days the months give, each part in RFC 7529 4.1's order.
func (p *plan) pick(f frame, ms []month) []rscale.Day {
	var days []rscale.Day
	if p.weeks != nil || p.yearDays != nil {
		days = p.ordinals(f)
		if p.monthDays != nil {
			days = slices.DeleteFunc(days, func(d rscale.Day) bool { return !p.onMonthDay(f, d) })
		}
	} else {
		for _, m := range ms {
			if p.monthDays == nil {
				for d := m.start; d < m.end(); d++ {
					days = append(days, d)
				}
				continue
			}
			for _, n := range p.monthDays {
				if i, ok := p.skip.place(n, m.days); ok {
					days = append(days, m.start+rscale.Day(i))
				}
			}
		}
	}
	days = slices.DeleteFunc(days, func(d rscale.Day) bool {
		_, ok := f.locate(d)
		return !ok || !p.onWeekday(f, d)
	})
	slices.Sort(days)
	return slices.Compact(days)
}

// ordinals is the days BYWEEKNO and BYYEARDAY name. BYMONTH limits them,
// in whichever year a week puts them, and BYWEEKNO limits BYYEARDAY,
// except where SKIP moved a day.
func (p *plan) ordinals(f frame) []rscale.Day {
	named := map[int][]month{}
	for _, m := range f.months {
		if _, ok := named[m.year]; !ok {
			named[m.year] = p.monthsOf(f, m.year)
		}
	}
	inMonths := func(d rscale.Day) bool {
		i, ok := f.locate(d)
		return p.months == nil || ok && slices.Contains(named[f.months[i].year], f.months[i])
	}
	var weekDays []rscale.Day
	if p.weeks != nil {
		first := weekOne(f.start, p.wkst)
		count := int(weekOne(f.end, p.wkst)-first) / 7
		for _, n := range p.weeks {
			i, ok := p.skip.place(n, count)
			if !ok {
				continue
			}
			for k := range 7 {
				if d := first + rscale.Day(7*i+k); beyond(n, count) || inMonths(d) {
					weekDays = append(weekDays, d)
				}
			}
		}
		if p.yearDays == nil {
			return weekDays
		}
	}
	var days []rscale.Day
	count := int(f.end - f.start)
	for _, n := range p.yearDays {
		i, ok := p.skip.place(n, count)
		d := f.start + rscale.Day(i)
		if ok && (beyond(n, count) || inMonths(d) && (p.weeks == nil || slices.Contains(weekDays, d))) {
			days = append(days, d)
		}
	}
	return days
}

// weekOne is the first day of week 1 of the year starting on start: the
// week holding the year's fourth day (RFC 5545 3.3.10, BYWEEKNO).
func weekOne(start rscale.Day, wkst time.Weekday) rscale.Day {
	return weekStart(start+3, wkst)
}

func weekStart(d rscale.Day, wkst time.Weekday) rscale.Day {
	return d - rscale.Day((int(d.Weekday())-int(wkst)+7)%7)
}

// onMonthDay limits by BYMONTHDAY: whether d's month, or one either side
// that SKIP moves a day from, names it.
func (p *plan) onMonthDay(f frame, d rscale.Day) bool {
	i, ok := f.locate(d)
	if !ok {
		return false
	}
	for _, m := range f.months[max(i-1, 0):min(i+2, len(f.months))] {
		for _, n := range p.monthDays {
			if k, ok := p.skip.place(n, m.days); ok && m.start+rscale.Day(k) == d {
				return true
			}
		}
	}
	return false
}

// onWeekday limits by BYDAY. A numbered one counts within the month for
// FREQ=MONTHLY or with BYMONTH, else within the year.
func (p *plan) onWeekday(f frame, d rscale.Day) bool {
	if p.days == nil {
		return true
	}
	for _, w := range p.days {
		if w.Day != d.Weekday() {
			continue
		}
		if w.N == 0 {
			return true
		}
		i, _ := f.locate(d)
		start, end := f.months[i].start, f.months[i].end()
		if p.freq != Monthly && p.months == nil {
			year := f.months[i].year
			first := slices.IndexFunc(f.months, func(m month) bool { return m.year == year })
			last := first
			for last+1 < len(f.months) && f.months[last+1].year == year {
				last++
			}
			start, end = f.months[first].start, f.months[last].end()
		}
		if w.N > 0 && int(d-start)/7 == w.N-1 || w.N < 0 && int(end-1-d)/7 == -w.N-1 {
			return true
		}
	}
	return false
}

// chosen applies BYSETPOS to a period's candidates: each day at each
// second of the clock, in order.
func (p *plan) chosen(days []rscale.Day, clock []int) []dayClock {
	var out []dayClock
	if p.setPos == nil {
		for _, d := range days {
			out = append(out, dayClock{d, clock})
		}
		return out
	}
	n := len(days) * len(clock)
	var picks []int
	for _, pos := range p.setPos {
		i := pos - 1
		if pos < 0 {
			i = n + pos
		}
		if i >= 0 && i < n {
			picks = append(picks, i)
		}
	}
	slices.Sort(picks)
	for _, i := range slices.Compact(picks) {
		d, s := days[i/len(clock)], clock[i%len(clock)]
		if k := len(out) - 1; k >= 0 && out[k].day == d {
			out[k].secs = append(out[k].secs, s)
		} else {
			out = append(out, dayClock{d, []int{s}})
		}
	}
	return out
}

// walk yields the rule's candidates in order, from the period holding
// from to the last that can reach stop. It ends where the calendar ends,
// so every rule ends.
func (p *plan) walk(from, stop rscale.Day) iter.Seq[dayClock] {
	return func(yield func(dayClock) bool) {
		switch p.freq {
		case Yearly:
			p.yearly(from, stop, yield)
		case Monthly:
			p.monthly(from, stop, yield)
		case Weekly:
			p.weekly(from, stop, yield)
		default:
			p.fine(from, stop, yield)
		}
	}
}

// A year's or month's days may reach into the next period's, so they
// are held until no later period can come before them.

func (p *plan) yearly(from, stop rscale.Day, yield func(dayClock) bool) {
	y := p.year0
	if fy, ok := p.cal.YearOf(from); ok && fy-1 > y {
		y += (fy - 1 - y) / p.interval * p.interval
	}
	var held []dayClock
	for ; ; y += p.interval {
		f, ok := p.frame(y)
		if !ok || f.start-reach > stop {
			break
		}
		if f.end+reach < from {
			continue
		}
		held = merge(held, p.chosen(p.pick(f, p.monthsOf(f, f.year)), p.clock))
		if !release(&held, f.end-reach, yield) {
			return
		}
	}
	release(&held, never, yield)
}

func (p *plan) monthly(from, stop rscale.Day, yield func(dayClock) bool) {
	var held []dayClock
	next := -p.month0 // months from DTSTART's to the first of year y
	for y := p.year0; ; y++ {
		year, ok := p.cal.Year(y)
		if !ok || year.Start-reach > stop {
			break
		}
		n := len(year.Months)
		k := max(next, 0)
		k += (p.interval - k%p.interval) % p.interval
		if k < next+n && year.End()+reach >= from {
			f, _ := p.frame(y)
			named := p.monthsOf(f, f.year)
			lo, hi := f.own(y)
			for i, m := range f.months[lo:hi] {
				k := next + i
				if k < 0 || k%p.interval != 0 || m.end()+reach < from || p.months != nil && !slices.Contains(named, m) {
					continue
				}
				held = merge(held, p.chosen(p.pick(f, []month{m}), p.clock))
				if !release(&held, m.end()-reach, yield) {
					return
				}
			}
		}
		next += n
	}
	release(&held, never, yield)
}

func (p *plan) weekly(from, stop rscale.Day, yield func(dayClock) bool) {
	first := weekStart(p.start, p.wkst)
	from = first + (from-first)/7*7
	var group []rscale.Day
	week := -1
	flush := func() bool {
		for _, dc := range p.chosen(group, p.clock) {
			if !yield(dc) {
				return false
			}
		}
		group = group[:0]
		return true
	}
	// A week is taken whole, so that BYSETPOS counts all of it.
	for d := range p.allowed(from, stop+7) {
		w := int(d-first) / 7
		if w%p.interval != 0 {
			continue
		}
		if w != week && !flush() {
			return
		}
		week = w
		group = append(group, d)
	}
	flush()
}

func (p *plan) fine(from, stop rscale.Day, yield func(dayClock) bool) {
	unit := int64(p.freq.seconds())
	perDay := 86400 / unit
	interval := int64(p.interval)
	first := floorDiv(int64(p.start)*86400+int64(p.startSec), unit)
	// A period's number within its day steps by perDay mod interval from
	// day to day, so a block whose number no day can reach never matches.
	if !slices.ContainsFunc(p.blocks, func(b block) bool {
		return floorMod(int64(b.unit)-first, gcd(perDay, interval)) == 0
	}) {
		return
	}
	byUnit := map[int64][]int{}
	for _, b := range p.blocks {
		byUnit[int64(b.unit)] = b.secs
	}
	// A day is matched by its blocks or by its periods, whichever are fewer.
	fewerBlocks := int64(len(p.blocks)) <= perDay/interval+1
	for d := range p.allowed(from, stop) {
		base := int64(d) * perDay
		dc := dayClock{day: d}
		if fewerBlocks {
			for _, b := range p.blocks {
				if floorMod(base+int64(b.unit)-first, interval) == 0 {
					dc.secs = append(dc.secs, b.secs...)
				}
			}
		} else {
			for n := base + floorMod(first-base, interval); n < base+perDay; n += interval {
				dc.secs = append(dc.secs, byUnit[n-base]...)
			}
		}
		if len(dc.secs) > 0 && !yield(dc) {
			return
		}
	}
}

// allowed yields the days from from to stop that the rule's day parts
// let through, a year at a time, with the days SKIP moves in from the
// years either side.
func (p *plan) allowed(from, stop rscale.Day) iter.Seq[rscale.Day] {
	return func(yield func(rscale.Day) bool) {
		y, ok := p.cal.YearOf(from)
		if !ok {
			return
		}
		sets := func(y int) (frame, []rscale.Day, bool) {
			f, ok := p.frame(y)
			if !ok {
				return f, nil, false
			}
			return f, p.pick(f, p.monthsOf(f, f.year)), true
		}
		_, prev, _ := sets(y - 1)
		f, cur, _ := sets(y)
		for {
			next, days, more := sets(y + 1)
			all := slices.Concat(prev, cur, days)
			all = slices.DeleteFunc(all, func(d rscale.Day) bool { return d < max(from, f.start) || d >= f.end })
			slices.Sort(all)
			for _, d := range slices.Compact(all) {
				if d > stop || !yield(d) {
					return
				}
			}
			if !more || f.end > stop {
				return
			}
			prev, cur, f, y = cur, days, next, y+1
		}
	}
}

// merge joins two runs of days in order, a day in both at the seconds of
// either.
func merge(a, b []dayClock) []dayClock {
	out := make([]dayClock, 0, len(a)+len(b))
	for len(a) > 0 || len(b) > 0 {
		switch {
		case len(b) == 0 || len(a) > 0 && a[0].day < b[0].day:
			out, a = append(out, a[0]), a[1:]
		case len(a) == 0 || b[0].day < a[0].day:
			out, b = append(out, b[0]), b[1:]
		default:
			secs := slices.Concat(a[0].secs, b[0].secs)
			slices.Sort(secs)
			out = append(out, dayClock{a[0].day, slices.Compact(secs)})
			a, b = a[1:], b[1:]
		}
	}
	return out
}

// release yields the held days before bound.
func release(held *[]dayClock, bound rscale.Day, yield func(dayClock) bool) bool {
	n := 0
	for n < len(*held) && (*held)[n].day < bound {
		if !yield((*held)[n]) {
			return false
		}
		n++
	}
	*held = (*held)[n:]
	return true
}

func floorDiv(a, b int64) int64 {
	q := a / b
	if a%b < 0 {
		q--
	}
	return q
}

func floorMod(a, b int64) int64 { return a - floorDiv(a, b)*b }

func gcd(a, b int64) int64 {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}
