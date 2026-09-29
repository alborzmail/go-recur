package recur

import (
	"bufio"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/alborzmail/go-recur/rscale"
)

// vector is one case in libical's format: a rule, its DTSTART and the
// instances it gives, from START-AT on where one is given.
type vector struct {
	id, name  string
	rule      string
	start     Value
	startAt   time.Time
	instances string
	exdate    []Value
	loc       *time.Location
}

func readVectors(t testing.TB, path string) []vector {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var out []vector
	var v vector
	flush := func() {
		if v.rule != "" {
			out = append(out, v)
		}
		v = vector{}
	}
	s := bufio.NewScanner(f)
	s.Buffer(nil, 1<<20)
	for n := 1; s.Scan(); n++ {
		line := s.Text()
		if line == "" {
			flush()
			continue
		}
		if name, ok := strings.CutPrefix(line, "#"); ok {
			v.name = strings.TrimSpace(strings.TrimSpace(v.name) + " " + strings.TrimSpace(name))
			continue
		}
		head, value, _ := strings.Cut(line, ":")
		key, params, _ := strings.Cut(head, ";")
		switch key {
		case "RRULE":
			v.rule = value
			v.id = fmt.Sprintf("%s:%d", filepath.Base(path), n)
		case "DTSTART":
			v.loc = time.UTC
			if tzid, ok := strings.CutPrefix(params, "TZID="); ok {
				if v.loc, err = time.LoadLocation(tzid); err != nil {
					t.Fatal(err)
				}
			}
			v.start = vectorValue(t, value, v.loc)
		case "START-AT":
			v.startAt = vectorValue(t, value, v.loc).in(v.loc)
		case "INSTANCES":
			v.instances = value
		case "EXDATE":
			for s := range strings.SplitSeq(value, ",") {
				v.exdate = append(v.exdate, vectorValue(t, s, v.loc))
			}
		default:
			t.Fatalf("%s:%d: unknown line %q", path, n, line)
		}
	}
	if err := s.Err(); err != nil {
		t.Fatal(err)
	}
	flush()
	return out
}

// vectorValue reads a time as the vector files write it: with a TZID a
// local time is zoned, without one floating.
func vectorValue(t testing.TB, s string, loc *time.Location) Value {
	t.Helper()
	v, err := ParseValue(s)
	if err != nil {
		t.Fatal(err)
	}
	if v.Kind == Floating && loc != time.UTC {
		v = Value{Time: v.in(loc)}
	}
	return v
}

// want is what a vector's rule gives here, where that is not what the
// file says: a strict refusal (nil) or a different reading.
type want struct {
	instances []string // nil for a rule Parse or the set refuses
	why       string
}

// The vectors this module reads otherwise, and why. A vector's
// instances lack DTSTART where the rule does not give it; here DTSTART
// is always the first (RFC 5545 3.8.5.3), so the runner adds it and
// drops the last beyond COUNT, and that difference is not listed.
var disagree = map[string]want{
	"rfc5545.txt:175": {
		instances: []string{"19970902T090000", "19970902T120000"},
		why:       "UNTIL=19970902T170000Z is 13:00 in New York; the RFC lists 15:00 as if it were local",
	},
	"libical_icalrecur_test.txt:403": {why: "a time of day in a rule on a date is refused, not ignored"},
	"libical_icalrecur_test.txt:408": {
		instances: []string{"20241230", "20251230", "20301230", "20311230"},
		why:       "libical does not implement BYMONTH, BYMONTHDAY and BYWEEKNO together",
	},
	"libical_icalrecur_test.txt:393": {
		instances: []string{"20240229", "20240329", "20240429"},
		why:       "BYMONTHDAY alone in a yearly rule names that day of every month, as BYDAY alone names every such weekday",
	},
	"libical_icalrecur_test.txt:398": {
		instances: []string{"20240201", "20240229", "20240303", "20240331"},
		why:       "BYMONTHDAY alone in a yearly rule names that day of every month",
	},
	"dateutil_rrule.txt:116": {
		instances: []string{"19970902T090000Z", "19980512T090000Z", "19990518T090000Z"},
		why:       "a week named without its day takes DTSTART's weekday, as a month named alone takes its day",
	},
}

// refused says which part strict parsing refuses in a vector; one
// reason covers every vector in its family.
func refused(parts map[string]string) (string, bool) {
	freq := parts["FREQ"]
	has := func(name string) bool { _, ok := parts[name]; return ok }
	numbered := slices.ContainsFunc(strings.Split(parts["BYDAY"], ","), func(s string) bool { return len(s) > 2 })
	switch {
	case has("COUNT") && has("UNTIL"):
		return "COUNT with UNTIL", true
	case has("BYSETPOS") && !slices.ContainsFunc(slices.Collect(maps.Keys(parts)), func(name string) bool {
		return strings.HasPrefix(name, "BY") && name != "BYSETPOS"
	}):
		return "BYSETPOS without another BYxxx", true
	case has("BYEASTER"):
		return "BYEASTER is not iCalendar", true
	case has("BYWEEKNO") && freq != "YEARLY":
		return "BYWEEKNO without FREQ=YEARLY", true
	case has("BYYEARDAY") && (freq == "DAILY" || freq == "WEEKLY" || freq == "MONTHLY"):
		return "BYYEARDAY with FREQ=DAILY, WEEKLY or MONTHLY", true
	case has("BYMONTHDAY") && freq == "WEEKLY":
		return "BYMONTHDAY with FREQ=WEEKLY", true
	case numbered && freq != "MONTHLY" && freq != "YEARLY":
		return "numbered BYDAY outside FREQ=MONTHLY or YEARLY", true
	}
	return "", false
}

func TestVectors(t *testing.T) {
	files, _ := filepath.Glob("testdata/*.txt")
	counts := map[string]int{}
	for _, file := range files {
		for _, v := range readVectors(t, file) {
			t.Run(v.id, func(t *testing.T) {
				counts[runVector(t, v)]++
			})
		}
	}
	if n := counts["matched"] + counts["matched with DTSTART first"]; n < 300 {
		t.Errorf("only %d vectors matched", n)
	}
	for k, n := range counts {
		t.Logf("%4d %s", n, k)
	}
}

// runVector checks one vector and says how it went.
func runVector(t *testing.T, v vector) string {
	rule, err := Parse(v.rule)
	var set Set
	if err == nil {
		set = Set{Start: v.start, Rule: &rule, ExDate: v.exdate}
		_, err = set.All()
	}
	expect, listed := disagree[v.id]
	parts := map[string]string{}
	for p := range strings.SplitSeq(strings.ToUpper(v.rule), ";") {
		name, value, _ := strings.Cut(p, "=")
		parts[name] = value
	}
	reason, strict := refused(parts)
	scale := parts["RSCALE"]
	_, known := rscale.Lookup(scale)
	switch {
	case scale != "" && !known:
		if !errors.Is(err, ErrScale) {
			t.Fatalf("%s: RSCALE=%s is not shipped, want ErrScale; got %v", v.name, scale, err)
		}
		return "unknown scale"
	case strict || listed && expect.instances == nil:
		if !errors.Is(err, ErrSyntax) {
			t.Fatalf("%s: %s%s, want ErrSyntax; got %v", v.name, reason, expect.why, err)
		}
		return "refused: " + reason + expect.why
	case err != nil:
		t.Fatalf("%s: %v", v.name, err)
	}

	result := "matched"
	var exp []time.Time
	if listed {
		result = "differs"
		for _, s := range expect.instances {
			exp = append(exp, vectorValue(t, s, v.loc).in(v.loc))
		}
	} else {
		for s := range strings.SplitSeq(v.instances, ",") {
			if s != "" {
				exp = append(exp, vectorValue(t, s, v.loc).in(v.loc))
			}
		}
		start := v.start.in(v.loc)
		if !slices.ContainsFunc(v.exdate, func(x Value) bool { return x.in(v.loc).Equal(start) }) &&
			(len(exp) == 0 || !exp[0].Equal(start)) {
			exp = slices.Insert(exp, 0, start)
			result = "matched with DTSTART first"
			if rule.Count > 0 && len(exp) > rule.Count {
				exp = exp[:rule.Count]
			}
		}
	}
	bounded := rule.Count > 0 || rule.Until != nil
	seq, _ := set.All()
	if !v.startAt.IsZero() {
		exp = slices.DeleteFunc(exp, func(x time.Time) bool { return x.Before(v.startAt) })
		seq, _ = set.Between(v.startAt, time.Date(9999, 12, 31, 0, 0, 0, 0, time.UTC))
	}
	var got []time.Time
	for x := range seq {
		if !bounded && len(got) == len(exp) || len(got) > len(exp)+10 {
			break
		}
		got = append(got, x)
	}
	if !slices.EqualFunc(got, exp, time.Time.Equal) {
		t.Fatalf("%s\n%s\nDTSTART %s\ngot  %s\nwant %s", v.name, v.rule, v.start.Time, format(got, v.loc), format(exp, v.loc))
	}
	return result
}

func format(ts []time.Time, loc *time.Location) string {
	s := make([]string, len(ts))
	for i, x := range ts {
		s[i] = x.In(loc).Format("20060102T150405")
	}
	return strings.Join(s, ",")
}
