package recur

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestParseRefuses(t *testing.T) {
	for _, s := range []string{
		"",
		"COUNT=3",
		"FREQ=FORTNIGHTLY",
		"FREQ=DAILY;FREQ=DAILY",
		"FREQ=DAILY;COUNT=",
		"FREQ=DAILY;;COUNT=2",
		"FREQ=DAILY;X-NAME=1",
		"FREQ=YEARLY;BYEASTER=0",
		"FREQ=DAILY;COUNT=2;UNTIL=20250101",
		"FREQ=DAILY;COUNT=0",
		"FREQ=DAILY;COUNT=-1",
		"FREQ=DAILY;COUNT=+1",
		"FREQ=DAILY;INTERVAL=0",
		"FREQ=DAILY;UNTIL=20250230",
		"FREQ=DAILY;UNTIL=2025-01-01",
		"FREQ=DAILY;UNTIL=20250101T250000Z",
		"FREQ=MONTHLY;SKIP=BACKWARD",
		"RSCALE=GREGORIAN;FREQ=MONTHLY;SKIP=SIDEWAYS",
		"FREQ=MONTHLY;BYWEEKNO=1",
		"FREQ=MONTHLY;BYYEARDAY=1",
		"FREQ=WEEKLY;BYMONTHDAY=1",
		"FREQ=DAILY;BYDAY=1MO",
		"FREQ=YEARLY;BYWEEKNO=1;BYDAY=1MO",
		"FREQ=MONTHLY;BYSETPOS=1",
		"FREQ=DAILY;BYSECOND=61",
		"FREQ=DAILY;BYMINUTE=60",
		"FREQ=DAILY;BYHOUR=24",
		"FREQ=DAILY;BYHOUR=-1",
		"FREQ=MONTHLY;BYMONTHDAY=0",
		"FREQ=MONTHLY;BYMONTHDAY=32",
		"FREQ=MONTHLY;BYMONTHDAY=-32",
		"FREQ=YEARLY;BYYEARDAY=367",
		"FREQ=YEARLY;BYWEEKNO=54",
		"FREQ=YEARLY;BYMONTH=13",
		"FREQ=YEARLY;BYMONTH=5L",
		"RSCALE=PERSIAN;FREQ=YEARLY;BYMONTH=5L",
		"FREQ=MONTHLY;BYDAY=0MO",
		"FREQ=YEARLY;BYDAY=54MO",
		"FREQ=MONTHLY;BYDAY=MON",
		"FREQ=MONTHLY;BYDAY=MO,",
		"FREQ=MONTHLY;BYDAY=1MO;BYSETPOS=0",
		"FREQ=MONTHLY;BYDAY=1MO;BYSETPOS=367",
		"FREQ=WEEKLY;WKST=XX",
	} {
		if r, err := Parse(s); !errors.Is(err, ErrSyntax) {
			t.Errorf("Parse(%q) = %v, %v; want ErrSyntax", s, r, err)
		}
	}
	for _, s := range []string{"RSCALE=HEBREW;FREQ=YEARLY", "RSCALE=RUSSIAN;FREQ=DAILY"} {
		if r, err := Parse(s); !errors.Is(err, ErrScale) || errors.Is(err, ErrSyntax) {
			t.Errorf("Parse(%q) = %v, %v; want ErrScale", s, r, err)
		}
	}
}

// A rule reads back as written, up to the order of its parts and case.
func TestParseString(t *testing.T) {
	for _, s := range []string{
		"FREQ=DAILY",
		"FREQ=DAILY;INTERVAL=1",
		"FREQ=WEEKLY;WKST=MO",
		"RSCALE=GREGORIAN;FREQ=YEARLY;SKIP=OMIT",
		"RSCALE=PERSIAN;FREQ=MONTHLY;BYMONTHDAY=31,-1;SKIP=BACKWARD",
		"FREQ=YEARLY;UNTIL=20251231;BYDAY=-1FR,MO,20TH",
		"FREQ=MONTHLY;UNTIL=20251231T235959;BYMONTH=5,4",
		"FREQ=SECONDLY;UNTIL=20251231T235959Z;BYSECOND=60,0;BYMINUTE=59;BYHOUR=23,0",
		"FREQ=YEARLY;COUNT=3;BYWEEKNO=-1,53;BYYEARDAY=-366,1;BYSETPOS=-1,1;WKST=SU",
		"freq=monthly;bymonthday=1,1;byday=su",
		"BYDAY=MO;FREQ=WEEKLY;COUNT=2",
	} {
		r, err := Parse(s)
		if err != nil {
			t.Fatalf("Parse(%q): %v", s, err)
		}
		if got := r.String(); !slices.Equal(parts(got), parts(s)) {
			t.Errorf("Parse(%q).String() = %q", s, got)
		}
	}
	if r, _ := Parse("FREQ=DAILY;"); r.String() != "FREQ=DAILY" {
		t.Errorf("a trailing semicolon is not read: %v", r)
	}
}

func parts(s string) []string {
	p := strings.Split(strings.ToUpper(s), ";")
	slices.Sort(p)
	return p
}

func FuzzParse(f *testing.F) {
	files, _ := filepath.Glob("testdata/*.txt")
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			f.Fatal(err)
		}
		for line := range strings.Lines(string(data)) {
			if rule, ok := strings.CutPrefix(strings.TrimSpace(line), "RRULE:"); ok {
				f.Add(rule)
			}
		}
	}
	f.Fuzz(func(t *testing.T, s string) {
		r, err := Parse(s)
		if err != nil {
			return
		}
		written := r.String()
		again, err := Parse(written)
		if err != nil {
			t.Fatalf("Parse(%q) refuses %q written from it: %v", s, written, err)
		}
		if !reflect.DeepEqual(again, r) || again.String() != written {
			t.Fatalf("%q reads back as %#v, not %#v", written, again, r)
		}
	})
}
