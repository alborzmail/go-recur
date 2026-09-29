package recur

import (
	"errors"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/alborzmail/go-recur/rscale"
)

func zone(t testing.TB, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

func collect(t testing.TB, s Set) []time.Time {
	t.Helper()
	seq, err := s.All()
	if err != nil {
		t.Fatal(err)
	}
	return slices.Collect(seq)
}

func mustParse(t testing.TB, s string) *Rule {
	t.Helper()
	r, err := Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	return &r
}

func utc(s string) time.Time {
	t, err := time.Parse("20060102T150405Z", s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestZones(t *testing.T) {
	ny, tehran := zone(t, "America/New_York"), zone(t, "Asia/Tehran")
	for _, c := range []struct {
		name  string
		rule  string
		start time.Time
		want  []string // UTC
	}{
		{"a time the spring gap skips takes the offset before it", "FREQ=DAILY;COUNT=3",
			time.Date(2025, 3, 8, 2, 30, 0, 0, ny), []string{"20250308T073000Z", "20250309T073000Z", "20250310T063000Z"}},
		{"a time the autumn repeats is its first", "FREQ=DAILY;COUNT=3",
			time.Date(2025, 11, 1, 1, 30, 0, 0, ny), []string{"20251101T053000Z", "20251102T053000Z", "20251103T063000Z"}},
		{"hourly steps the wall clock, so the repeated hour comes once", "FREQ=HOURLY;COUNT=4",
			time.Date(2025, 11, 2, 0, 0, 0, 0, ny), []string{"20251102T040000Z", "20251102T050000Z", "20251102T070000Z", "20251102T080000Z"}},
		{"hourly over the skipped hour gives its instant once", "FREQ=HOURLY;COUNT=4",
			time.Date(2025, 3, 9, 0, 0, 0, 0, ny), []string{"20250309T050000Z", "20250309T060000Z", "20250309T070000Z", "20250309T080000Z"}},
		{"Tehran's gap at midnight on 2 Farvardin", "FREQ=DAILY;COUNT=3",
			time.Date(2020, 3, 20, 0, 30, 0, 0, tehran), []string{"20200319T210000Z", "20200320T210000Z", "20200321T200000Z"}},
		{"Tehran's repeated hour before midnight on 30 Shahrivar", "FREQ=DAILY;COUNT=3",
			time.Date(2020, 9, 19, 23, 30, 0, 0, tehran), []string{"20200919T190000Z", "20200920T190000Z", "20200921T200000Z"}},
		{"Nowruz at noon in Tehran, over the years DST ended", "RSCALE=PERSIAN;FREQ=YEARLY;COUNT=4",
			time.Date(2020, 3, 20, 12, 0, 0, 0, tehran), []string{"20200320T083000Z", "20210321T083000Z", "20220321T083000Z", "20230321T083000Z"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := collect(t, Set{Start: Value{Time: c.start}, Rule: mustParse(t, c.rule)})
			var want []time.Time
			for _, s := range c.want {
				want = append(want, utc(s))
			}
			if !slices.EqualFunc(got, want, time.Time.Equal) {
				t.Errorf("got %v\nwant %v", got, want)
			}
			for _, x := range got {
				if x.Location() != c.start.Location() {
					t.Errorf("%v is not in DTSTART's location", x)
				}
			}
		})
	}
}

// Real data carries an UNTIL whose type is not DTSTART's; it is read as
// RFC 5545 means it.
func TestUntilTypes(t *testing.T) {
	ny := zone(t, "America/New_York")
	for _, c := range []struct {
		name  string
		rule  string
		start Value
		n     int
	}{
		{"a date on a zoned start ends with that day", "FREQ=DAILY;UNTIL=20250105",
			Value{Time: time.Date(2025, 1, 1, 23, 0, 0, 0, ny)}, 5},
		{"a floating time on a zoned start is read in its zone", "FREQ=DAILY;UNTIL=20250103T090000",
			Value{Time: time.Date(2025, 1, 1, 9, 0, 0, 0, ny)}, 3},
		{"UTC on a floating start", "FREQ=DAILY;UNTIL=20250103T140000Z",
			Value{time.Date(2025, 1, 1, 9, 0, 0, 0, ny), Floating}, 3},
		{"a date-time on a date", "FREQ=DAILY;UNTIL=20250103T000000Z",
			Value{time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), Date}, 3},
		{"a date on a date", "FREQ=DAILY;UNTIL=20250103",
			Value{time.Date(2025, 1, 1, 0, 0, 0, 0, ny), Date}, 3},
		{"DTSTART after UNTIL is still the first", "FREQ=DAILY;UNTIL=20241231",
			Value{time.Date(2025, 1, 1, 0, 0, 0, 0, ny), Date}, 1},
	} {
		if got := collect(t, Set{Start: c.start, Rule: mustParse(t, c.rule)}); len(got) != c.n {
			t.Errorf("%s: %d instances %v; want %d", c.name, len(got), got, c.n)
		}
	}
}

func TestSetRefuses(t *testing.T) {
	date := Value{time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), Date}
	for _, c := range []struct {
		set  Set
		want error
	}{
		{Set{Start: date, Rule: mustParse(t, "FREQ=HOURLY")}, ErrSyntax},
		{Set{Start: date, Rule: mustParse(t, "FREQ=DAILY;BYHOUR=9")}, ErrSyntax},
		{Set{Start: date, Rule: &Rule{Freq: Daily, Interval: -1}}, ErrSyntax},
		{Set{Start: date, Rule: &Rule{Scale: "HEBREW", Freq: Yearly}}, ErrScale},
		{Set{Start: Value{Time: time.Date(3800, 1, 1, 0, 0, 0, 0, time.UTC)}, Rule: mustParse(t, "RSCALE=PERSIAN;FREQ=YEARLY")}, ErrScale},
	} {
		if _, err := c.set.All(); !errors.Is(err, c.want) {
			t.Errorf("%v: %v; want %v", c.set.Rule, err, c.want)
		}
	}
}

// A rule that can never match ends where its calendar does: no rule
// runs forever, and none needs a cap on barren periods.
func TestImpossibleRulesEnd(t *testing.T) {
	start := Value{Time: time.Date(2025, 1, 6, 0, 0, 0, 0, time.UTC)} // a Monday
	for _, rule := range []string{
		"FREQ=YEARLY;BYMONTH=2;BYMONTHDAY=31",
		"FREQ=YEARLY;BYWEEKNO=53;BYMONTH=6",
		"FREQ=YEARLY;BYYEARDAY=60;BYMONTH=1",
		"FREQ=MONTHLY;BYMONTH=2;BYMONTHDAY=30",
		"FREQ=MONTHLY;BYDAY=6MO",
		"FREQ=WEEKLY;BYMONTH=2;BYDAY=MO;BYSETPOS=2",
		"FREQ=DAILY;BYMONTH=4;BYMONTHDAY=31",
		"FREQ=DAILY;INTERVAL=14;BYDAY=TU",
		"FREQ=HOURLY;BYMONTH=2;BYMONTHDAY=31",
		"FREQ=HOURLY;INTERVAL=48;BYHOUR=1",
		"FREQ=MINUTELY;BYMONTH=11;BYMONTHDAY=31",
		"FREQ=SECONDLY;BYMONTH=2;BYMONTHDAY=30",
		"FREQ=SECONDLY;INTERVAL=2;BYSECOND=1",
		"FREQ=SECONDLY;BYSECOND=60",
		"FREQ=MINUTELY;BYSECOND=1;BYSETPOS=3,-3",
		"FREQ=MINUTELY;INTERVAL=10080;BYDAY=TU",
		"RSCALE=PERSIAN;FREQ=YEARLY;BYMONTH=7;BYMONTHDAY=31",
		"RSCALE=PERSIAN;FREQ=DAILY;BYMONTH=12;BYMONTHDAY=31",
	} {
		t.Run(rule, func(t *testing.T) {
			begun := time.Now()
			got := collect(t, Set{Start: start, Rule: mustParse(t, rule)})
			if len(got) != 1 || !got[0].Equal(start.Time) {
				t.Errorf("got %v; want DTSTART alone", got)
			}
			t.Logf("ended in %v", time.Since(begun))
		})
	}
}

func TestRDateExDate(t *testing.T) {
	ny := zone(t, "America/New_York")
	day := func(d, h int) time.Time { return time.Date(2025, 1, d, h, 0, 0, 0, ny) }
	s := Set{
		Start: Value{Time: day(1, 9)},
		Rule:  mustParse(t, "FREQ=DAILY;COUNT=4"),
		RDate: []Period{
			{Start: Value{Time: day(10, 9)}},
			{Start: Value{Time: day(2, 9)}},
			{Start: Value{time.Date(2025, 1, 5, 12, 0, 0, 0, time.UTC), Floating}, End: Value{Time: day(5, 14)}},
		},
		ExDate: []Value{{Time: day(3, 9)}, {Time: day(1, 9)}},
	}
	want := []time.Time{day(2, 9), day(4, 9), day(5, 12), day(10, 9)}
	if got := collect(t, s); !slices.EqualFunc(got, want, time.Time.Equal) {
		t.Errorf("got %v\nwant %v", got, want)
	}
	seq, _ := s.Between(day(3, 0), day(10, 9))
	if got := slices.Collect(seq); !slices.EqualFunc(got, want[1:3], time.Time.Equal) {
		t.Errorf("Between got %v", got)
	}
	occs, _ := s.occurrences(window{})
	for o := range occs {
		if o.start.Equal(day(5, 12)) && !o.end.Equal(day(5, 14)) {
			t.Errorf("the RDATE period ends at %v", o.end)
		}
	}
	alone := Set{Start: Value{Time: day(1, 9)}, RDate: []Period{{Start: Value{Time: day(1, 9)}}}}
	if got := collect(t, alone); len(got) != 1 {
		t.Errorf("DTSTART and an RDATE at it give %v", got)
	}
}

// Between is All cut to its window, wherever the window falls.
func TestBetweenIsAllCut(t *testing.T) {
	ny := zone(t, "America/New_York")
	files, _ := filepath.Glob("testdata/*.txt")
	n := 0
	for _, file := range files {
		for _, v := range readVectors(t, file) {
			r, err := Parse(v.rule)
			if err != nil {
				continue
			}
			for _, loc := range []*time.Location{v.loc, ny} {
				s := Set{Start: Value{Time: v.start.in(loc), Kind: v.start.Kind}, Rule: &r}
				if s.Start.Kind == Floating {
					s.Start.Kind = DateTime
				}
				if _, err := s.All(); err != nil {
					continue
				}
				checkBetween(t, v.id, s)
				n++
			}
		}
	}
	if n < 600 {
		t.Errorf("checked only %d sets", n)
	}
}

func checkBetween(t *testing.T, id string, s Set) {
	t.Helper()
	seq, _ := s.All()
	var all []time.Time
	for x := range seq {
		if len(all) == 200 {
			break
		}
		all = append(all, x)
	}
	if !slices.IsSortedFunc(all, time.Time.Compare) || len(slices.CompactFunc(slices.Clone(all), time.Time.Equal)) != len(all) {
		t.Fatalf("%s: not in order or repeated: %v", id, all)
	}
	if len(all) < 2 {
		return
	}
	last := all[len(all)-1]
	for _, w := range [][2]int{{0, len(all) - 1}, {1, len(all) / 2}, {len(all) / 3, len(all) - 1}, {len(all) / 2, min(len(all)/2+1, len(all)-1)}} {
		from, to := all[w[0]].Add(-time.Second), all[w[1]]
		if w[0] > 0 {
			from = all[w[0]].Add(time.Second)
		}
		seq, _ := s.Between(from, to)
		got := slices.Collect(seq)
		want := slices.DeleteFunc(slices.Clone(all), func(x time.Time) bool { return x.Before(from) || !x.Before(to) || x.After(last) })
		if !slices.EqualFunc(got, want, time.Time.Equal) {
			t.Fatalf("%s: Between(%v, %v)\ngot  %v\nwant %v", id, from, to, got, want)
		}
	}
}

// The invariants every set keeps: instances in order without repeats,
// DTSTART first, no more than COUNT, none past UNTIL, and Between is All
// cut to its window.
func FuzzSet(f *testing.F) {
	files, _ := filepath.Glob("testdata/*.txt")
	for _, file := range files {
		for _, v := range readVectors(f, file) {
			f.Add(v.rule, v.start.Unix(), uint8(len(v.rule)))
		}
	}
	zones := []*time.Location{time.UTC}
	for _, name := range []string{"America/New_York", "Asia/Tehran", "Australia/Lord_Howe", "Pacific/Apia", "Europe/Berlin"} {
		zones = append(zones, zone(f, name))
	}
	f.Fuzz(func(t *testing.T, rule string, start int64, z uint8) {
		r, err := Parse(rule)
		if err != nil {
			return
		}
		first := time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
		start = first + (start%(200*365*86400)+200*365*86400)%(200*365*86400)
		s := Set{Start: Value{Time: time.Unix(start, 0).In(zones[int(z)%len(zones)])}, Rule: &r}
		seq, err := s.All()
		if err != nil {
			return
		}
		var all []time.Time
		for x := range seq {
			if len(all) == 200 {
				break
			}
			all = append(all, x)
		}
		want := at(rscale.DayOf(s.Start.Time), s.Start.clock(), s.Start.Location())
		if len(all) == 0 || !all[0].Equal(want) {
			t.Fatalf("%s from %v: first %v", rule, s.Start.Time, all)
		}
		if r.Count > 0 && len(all) > r.Count {
			t.Fatalf("%s: %d instances", rule, len(all))
		}
		if r.Until != nil && r.Until.Kind == DateTime && len(all) > 1 && all[len(all)-1].After(r.Until.Time) {
			t.Fatalf("%s: %v is past UNTIL", rule, all[len(all)-1])
		}
		checkBetween(t, rule, s)
	})
}

// SKIP moves a missing day where a part limits as well as where it
// expands.
func TestSkipWhereLimiting(t *testing.T) {
	start := Value{time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC), Date}
	for rule, want := range map[string][]string{
		"RSCALE=GREGORIAN;FREQ=DAILY;BYMONTHDAY=31;SKIP=BACKWARD;COUNT=4": {"20250131", "20250228", "20250331", "20250430"},
		"RSCALE=GREGORIAN;FREQ=DAILY;BYMONTHDAY=31;SKIP=FORWARD;COUNT=4":  {"20250131", "20250301", "20250331", "20250501"},
	} {
		var got []string
		for _, x := range collect(t, Set{Start: start, Rule: mustParse(t, rule)}) {
			got = append(got, x.Format("20060102"))
		}
		if !slices.Equal(got, want) {
			t.Errorf("%s: %v; want %v", rule, got, want)
		}
	}
}
