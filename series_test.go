package recur

import (
	"slices"
	"testing"
	"time"
)

func TestSeries(t *testing.T) {
	day := func(d, h, m int) time.Time { return time.Date(2025, 1, d, h, m, 0, 0, time.UTC) }
	s := Series{
		Set:      Set{Start: Value{Time: day(1, 9, 0)}, Rule: mustParse(t, "FREQ=DAILY;COUNT=5")},
		Duration: time.Hour,
		Overrides: []Override{
			{ID: day(3, 9, 0), Start: day(3, 15, 0), Duration: 2 * time.Hour},
			{ID: day(10, 9, 0), Start: day(10, 11, 0), Duration: time.Hour},
			{ID: day(4, 9, 0), Future: true, Start: day(4, 10, 0), Duration: 30 * time.Minute},
		},
	}
	want := []Instance{
		{day(1, 9, 0), day(1, 9, 0), day(1, 10, 0), -1},
		{day(2, 9, 0), day(2, 9, 0), day(2, 10, 0), -1},
		{day(3, 9, 0), day(3, 15, 0), day(3, 17, 0), 0},
		{day(4, 9, 0), day(4, 10, 0), day(4, 10, 30), 2},
		{day(5, 9, 0), day(5, 10, 0), day(5, 10, 30), 2},
		{day(10, 9, 0), day(10, 11, 0), day(10, 12, 0), 1},
	}
	check := func(from, to time.Time, want []Instance) {
		t.Helper()
		seq, err := s.Between(from, to)
		if err != nil {
			t.Fatal(err)
		}
		if got := slices.Collect(seq); !slices.Equal(got, want) {
			t.Errorf("Between(%v, %v)\ngot  %v\nwant %v", from, to, got, want)
		}
	}
	check(day(1, 0, 0), day(11, 0, 0), want)
	check(day(5, 9, 30), day(5, 10, 1), want[4:5])
	check(day(3, 16, 0), day(4, 10, 0), want[2:3])

	// A THISANDFUTURE override moves later instances into a window their IDs are outside of.
	s.Overrides = []Override{{ID: day(2, 9, 0), Future: true, Start: day(5, 9, 0), Duration: time.Hour}}
	check(day(6, 0, 0), day(7, 0, 0), []Instance{{day(3, 9, 0), day(6, 9, 0), day(6, 10, 0), 0}})
	check(day(2, 0, 0), day(3, 0, 0), nil)
}

// An instance without duration is in a window it starts in (RFC 4791 9.9).
func TestSeriesInstant(t *testing.T) {
	start := time.Date(2025, 1, 1, 9, 0, 0, 0, time.UTC)
	s := Series{Set: Set{Start: Value{Time: start}}}
	for _, c := range []struct {
		from, to time.Time
		n        int
	}{
		{start, start.Add(time.Minute), 1},
		{start.Add(-time.Minute), start, 0},
		{start.Add(time.Second), start.Add(time.Minute), 0},
	} {
		seq, _ := s.Between(c.from, c.to)
		if got := slices.Collect(seq); len(got) != c.n {
			t.Errorf("Between(%v, %v) = %v", c.from, c.to, got)
		}
	}
}

// A window up to the last year a date can write is walked only as far as
// the caller reads.
func TestSeriesLazy(t *testing.T) {
	start := time.Date(2025, 1, 1, 9, 0, 0, 0, time.UTC)
	s := Series{Set: Set{Start: Value{Time: start}, Rule: mustParse(t, "FREQ=SECONDLY")}}
	seq, err := s.Between(start, time.Date(9999, 12, 31, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	for i := range seq {
		if !i.Start.Equal(start) {
			t.Errorf("first instance at %v, want %v", i.Start, start)
		}
		break
	}
}

// Overrides without their master, as an invitation to one instance
// carries them, are the series' only instances.
func TestSeriesWithoutMaster(t *testing.T) {
	at := time.Date(2025, 1, 3, 9, 0, 0, 0, time.UTC)
	s := Series{Overrides: []Override{{ID: at, Start: at, Duration: time.Hour}}}
	seq, err := s.Between(time.Time{}, at.AddDate(1, 0, 0))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := slices.Collect(seq), []Instance{{at, at, at.Add(time.Hour), 0}}; !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
