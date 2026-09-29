package recur

import (
	"cmp"
	"iter"
	"slices"
	"time"
)

// Series is a recurring component and the components that override its
// instances (RFC 5545 3.8.4.4).
type Series struct {
	Set       Set
	Duration  time.Duration
	Overrides []Override
}

type Override struct {
	ID       time.Time // RECURRENCE-ID
	Future   bool      // RANGE=THISANDFUTURE
	Start    time.Time
	Duration time.Duration
}

// Instance is one occurrence; Override indexes Overrides, -1 for the
// master.
type Instance struct {
	ID         time.Time
	Start, End time.Time
	Override   int
}

// Between yields the instances overlapping [from, to), by start. An
// override replaces the master's instance with its ID, and is an
// instance at its own start even where the master has none there. A
// THISANDFUTURE override moves every later instance by as much as it
// moves its own, and gives them its duration, up to the next one; they
// are its instances, since its properties hold for them.
func (s Series) Between(from, to time.Time) (iter.Seq[Instance], error) {
	var ahead, behind, longest time.Duration
	var futures []int
	for i, o := range s.Overrides {
		if o.Future {
			shift := o.Start.Sub(o.ID)
			ahead, behind = max(ahead, shift), min(behind, shift)
			longest = max(longest, o.Duration)
			futures = append(futures, i)
		}
	}
	slices.SortFunc(futures, func(a, b int) int { return s.Overrides[a].ID.Compare(s.Overrides[b].ID) })
	for _, r := range s.Set.RDate {
		if !r.End.IsZero() {
			longest = max(longest, r.End.Sub(r.Start.Time))
		}
	}
	longest = max(longest, s.Duration)
	// The master's instances that a shift or a duration can bring into the window.
	occs, err := s.Set.occurrences(window{from.Add(-ahead - longest), to.Add(-behind), true})
	if err != nil {
		return nil, err
	}
	return func(yield func(Instance) bool) {
		var out []Instance
		for o := range occs {
			if slices.ContainsFunc(s.Overrides, func(v Override) bool { return v.ID.Equal(o.start) }) {
				continue
			}
			inst := Instance{ID: o.start, Start: o.start, End: o.start.Add(s.Duration), Override: -1}
			if !o.end.IsZero() {
				inst.End = o.end
			}
			if k, _ := slices.BinarySearchFunc(futures, o.start, func(i int, t time.Time) int {
				return s.Overrides[i].ID.Compare(t)
			}); k > 0 {
				f := s.Overrides[futures[k-1]]
				inst.Start = o.start.Add(f.Start.Sub(f.ID))
				inst.End = inst.Start.Add(f.Duration)
				inst.Override = futures[k-1]
			}
			out = append(out, inst)
		}
		for i, o := range s.Overrides {
			out = append(out, Instance{o.ID, o.Start, o.Start.Add(o.Duration), i})
		}
		out = slices.DeleteFunc(out, func(i Instance) bool { return !overlaps(i, from, to) })
		slices.SortStableFunc(out, func(a, b Instance) int {
			return cmp.Or(a.Start.Compare(b.Start), a.ID.Compare(b.ID))
		})
		for _, i := range out {
			if !yield(i) {
				return
			}
		}
	}, nil
}

// overlaps is RFC 4791 9.9's test: an instance without duration overlaps
// where it starts.
func overlaps(i Instance, from, to time.Time) bool {
	if i.End.Equal(i.Start) {
		return !i.Start.Before(from) && i.Start.Before(to)
	}
	return i.Start.Before(to) && i.End.After(from)
}
