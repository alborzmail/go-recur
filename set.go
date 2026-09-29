package recur

import (
	"iter"
	"math"
	"slices"
	"time"

	"github.com/alborzmail/go-recur/rscale"
)

// Set is a recurrence set (RFC 5545 3.8.5): DTSTART, the rule's
// instances and RDATE's, less EXDATE's. The instants are read in
// Start's location.
type Set struct {
	Start  Value
	Rule   *Rule // nil for a set of RDATEs
	RDate  []Period
	ExDate []Value
}

// All yields the set's instants in order, without duplicates. DTSTART is
// the first and counts toward COUNT even where the rule does not give
// it; COUNT applies before EXDATE. The error is the set's own: a rule
// that is invalid or does not fit DTSTART, or a DTSTART its calendar
// cannot count.
func (s Set) All() (iter.Seq[time.Time], error) {
	return starts(s.occurrences(window{}))
}

// Between yields the instants in [from, to).
func (s Set) Between(from, to time.Time) (iter.Seq[time.Time], error) {
	return starts(s.occurrences(window{from, to, true}))
}

type window struct {
	from, to time.Time
	bounded  bool
}

// occurrence is an instant and, for an RDATE period, its end.
type occurrence struct {
	start, end time.Time
}

func starts(occs iter.Seq[occurrence], err error) (iter.Seq[time.Time], error) {
	return func(yield func(time.Time) bool) {
		for o := range occs {
			if !yield(o.start) {
				return
			}
		}
	}, err
}

func (s Set) occurrences(w window) (iter.Seq[occurrence], error) {
	loc := s.Start.Location()
	start := at(rscale.DayOf(s.Start.Time), s.Start.clock(), loc)
	rule := func(yield func(time.Time) bool) { yield(start) }
	if s.Rule != nil {
		p, err := newPlan(*s.Rule, s.Start)
		if err != nil {
			return nil, err
		}
		rule = p.instants(start, loc, w)
	}
	var rdates []occurrence
	for _, r := range s.RDate {
		o := occurrence{start: r.Start.in(loc)}
		if !r.End.IsZero() {
			o.end = r.End.in(loc)
		}
		rdates = append(rdates, o)
	}
	slices.SortStableFunc(rdates, func(a, b occurrence) int { return a.start.Compare(b.start) })
	rdates = slices.CompactFunc(rdates, func(a, b occurrence) bool { return a.start.Equal(b.start) })
	var exdates []time.Time
	for _, v := range s.ExDate {
		exdates = append(exdates, v.in(loc))
	}
	slices.SortFunc(exdates, time.Time.Compare)

	return func(yield func(occurrence) bool) {
		rdates, exdates := rdates, exdates
		out := func(o occurrence) bool {
			for len(exdates) > 0 && exdates[0].Before(o.start) {
				exdates = exdates[1:]
			}
			switch {
			case len(exdates) > 0 && exdates[0].Equal(o.start), w.bounded && o.start.Before(w.from):
				return true
			case w.bounded && !o.start.Before(w.to):
				return false
			}
			return yield(o)
		}
		for x := range rule {
			for len(rdates) > 0 && rdates[0].start.Before(x) {
				if !out(rdates[0]) {
					return
				}
				rdates = rdates[1:]
			}
			o := occurrence{start: x}
			if len(rdates) > 0 && rdates[0].start.Equal(x) {
				o, rdates = rdates[0], rdates[1:]
			}
			if !out(o) {
				return
			}
		}
		for _, o := range rdates {
			if !out(o) {
				return
			}
		}
	}, nil
}

// never is a day past every calendar's end, with room to add to it.
const never = rscale.Day(math.MaxInt / 2)

// margin widens the days a walk covers by one either side of the
// instants asked for, which a UTC offset can move by up to a day.
const margin = 1

// instants is the rule's instants from DTSTART, bounded by COUNT and
// UNTIL, from the start of w on where w is bounded.
func (p *plan) instants(start time.Time, loc *time.Location, w window) iter.Seq[time.Time] {
	from, stop := p.start, never
	if w.bounded {
		if p.count == 0 {
			from = max(from, rscale.DayOf(w.from.In(loc))-margin)
		}
		stop = rscale.DayOf(w.to.In(loc)) + margin
	}
	// UNTIL is inclusive; a date as a whole day, a floating time on the
	// wall clock of loc (RFC 5545 3.3.10, read leniently for the types
	// DTSTART does not match).
	var until time.Time
	if p.until != nil {
		until = p.until.in(loc).Add(time.Second)
		if p.until.Kind == Date {
			until = at(rscale.DayOf(p.until.Time)+1, 0, loc)
		}
		stop = min(stop, rscale.DayOf(until.In(loc))+margin)
	}
	return func(yield func(time.Time) bool) {
		n := 0
		emit := func(x time.Time) bool {
			n++
			return yield(x) && (p.count == 0 || n < p.count)
		}
		first := true
		for x := range ordered(p.walk(from, stop), loc) {
			if x.Before(start) {
				continue
			}
			if first {
				first = false
				if !emit(start) {
					return
				}
				if x.Equal(start) {
					continue
				}
			}
			if p.until != nil && !x.Before(until) || !emit(x) {
				return
			}
		}
		if first {
			emit(start)
		}
	}
}

// ordered turns wall clocks into instants in order: in a DST gap a wall
// clock can read later than a later one.
func ordered(days iter.Seq[dayClock], loc *time.Location) iter.Seq[time.Time] {
	return func(yield func(time.Time) bool) {
		var pending []time.Time
		for dc := range days {
			for _, s := range dc.secs {
				x := at(dc.day, s, loc)
				if i, found := slices.BinarySearchFunc(pending, x, time.Time.Compare); !found {
					pending = slices.Insert(pending, i, x)
				}
			}
			// No UTC offset reaches a day, so no later day's wall clock
			// reads earlier than this day began in UTC.
			bound := time.Unix(int64(dc.day)*86400, 0)
			n := 0
			for n < len(pending) && pending[n].Before(bound) {
				if !yield(pending[n]) {
					return
				}
				n++
			}
			pending = pending[n:]
		}
		for _, x := range pending {
			if !yield(x) {
				return
			}
		}
	}
}
