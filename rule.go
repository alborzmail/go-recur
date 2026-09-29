// Package recur expands iCalendar recurrences: RRULE, RDATE and EXDATE
// (RFC 5545) with RSCALE and SKIP (RFC 7529), and the instances that
// override them.
package recur

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/alborzmail/go-recur/rscale"
)

var (
	// ErrSyntax marks a value that is not a recurrence rule, or a rule
	// that does not fit its DTSTART.
	ErrSyntax = errors.New("recur: invalid rule")
	// ErrScale marks a rule in a calendar this module does not count, or
	// a DTSTART outside the years its calendar reckons.
	ErrScale = errors.New("recur: unknown calendar scale")
)

type Freq int

const (
	Secondly Freq = iota + 1
	Minutely
	Hourly
	Daily
	Weekly
	Monthly
	Yearly
)

func (f Freq) String() string {
	return [...]string{"", "SECONDLY", "MINUTELY", "HOURLY", "DAILY", "WEEKLY", "MONTHLY", "YEARLY"}[f]
}

// Skip says where a day or month a year lacks goes (RFC 7529 4.1).
// Zero is absent, which means Omit.
type Skip int

const (
	Omit Skip = iota + 1
	Backward
	Forward
)

func (s Skip) String() string {
	return [...]string{"", "OMIT", "BACKWARD", "FORWARD"}[s]
}

// WeekdayNum is a BYDAY entry: N is 0 for every such weekday.
type WeekdayNum struct {
	N   int
	Day time.Weekday
}

func (w WeekdayNum) String() string {
	if w.N == 0 {
		return weekday(w.Day)
	}
	return strconv.Itoa(w.N) + weekday(w.Day)
}

type Rule struct {
	Scale    string // RSCALE, upper case; "" when absent
	Freq     Freq
	Until    *Value
	Count    int // 0 when absent
	Interval int // 0 when absent, which means 1

	BySecond, ByMinute, ByHour []int
	ByDay                      []WeekdayNum
	ByMonthDay, ByYearDay      []int
	ByWeekNo                   []int
	ByMonth                    []rscale.Month
	BySetPos                   []int

	WeekStart *time.Weekday // nil when absent, which means Monday
	Skip      Skip
}

// Parse reads an RRULE value. It is strict: whatever RFC 5545 and RFC
// 7529 do not allow is an error.
func Parse(s string) (Rule, error) {
	var r Rule
	seen := map[string]bool{}
	// A trailing semicolon is common in the wild (libical has Google
	// Calendar's) and leaves the rule unambiguous.
	for part := range strings.SplitSeq(strings.TrimSuffix(s, ";"), ";") {
		name, value, _ := strings.Cut(part, "=")
		name = strings.ToUpper(name)
		if value == "" {
			return Rule{}, syntaxf("no value in %q", part)
		}
		if seen[name] {
			return Rule{}, syntaxf("%s twice", name)
		}
		seen[name] = true
		var err error
		switch name {
		case "RSCALE":
			r.Scale = strings.ToUpper(value)
		case "FREQ":
			r.Freq, err = parseName(value, Freq.String, Secondly, Yearly)
		case "UNTIL":
			var v Value
			v, err = ParseValue(value)
			r.Until = &v
		case "COUNT":
			r.Count, err = parsePositive(value)
		case "INTERVAL":
			r.Interval, err = parsePositive(value)
		case "BYSECOND":
			r.BySecond, err = parseList(value, parseUnsigned)
		case "BYMINUTE":
			r.ByMinute, err = parseList(value, parseUnsigned)
		case "BYHOUR":
			r.ByHour, err = parseList(value, parseUnsigned)
		case "BYDAY":
			r.ByDay, err = parseList(value, parseWeekdayNum)
		case "BYMONTHDAY":
			r.ByMonthDay, err = parseList(value, parseSigned)
		case "BYYEARDAY":
			r.ByYearDay, err = parseList(value, parseSigned)
		case "BYWEEKNO":
			r.ByWeekNo, err = parseList(value, parseSigned)
		case "BYMONTH":
			r.ByMonth, err = parseList(value, rscale.ParseMonth)
		case "BYSETPOS":
			r.BySetPos, err = parseList(value, parseSigned)
		case "WKST":
			var d time.Weekday
			d, err = parseWeekday(value)
			r.WeekStart = &d
		case "SKIP":
			r.Skip, err = parseName(value, Skip.String, Omit, Forward)
		default:
			return Rule{}, syntaxf("unknown part %s", name)
		}
		if err != nil {
			return Rule{}, fmt.Errorf("%w: %s: %w", ErrSyntax, name, err)
		}
	}
	return r, r.validate()
}

// String writes the rule's parts in one order, upper case.
func (r Rule) String() string {
	var parts []string
	add := func(name, value string) { parts = append(parts, name+"="+value) }
	if r.Scale != "" {
		add("RSCALE", r.Scale)
	}
	add("FREQ", r.Freq.String())
	if r.Until != nil {
		add("UNTIL", r.Until.String())
	}
	if r.Count != 0 {
		add("COUNT", strconv.Itoa(r.Count))
	}
	if r.Interval != 0 {
		add("INTERVAL", strconv.Itoa(r.Interval))
	}
	for _, l := range []struct {
		name   string
		values []int
	}{{"BYSECOND", r.BySecond}, {"BYMINUTE", r.ByMinute}, {"BYHOUR", r.ByHour}} {
		if l.values != nil {
			add(l.name, join(l.values, strconv.Itoa))
		}
	}
	if r.ByDay != nil {
		add("BYDAY", join(r.ByDay, WeekdayNum.String))
	}
	for _, l := range []struct {
		name   string
		values []int
	}{{"BYMONTHDAY", r.ByMonthDay}, {"BYYEARDAY", r.ByYearDay}, {"BYWEEKNO", r.ByWeekNo}} {
		if l.values != nil {
			add(l.name, join(l.values, strconv.Itoa))
		}
	}
	if r.ByMonth != nil {
		add("BYMONTH", join(r.ByMonth, rscale.Month.String))
	}
	if r.BySetPos != nil {
		add("BYSETPOS", join(r.BySetPos, strconv.Itoa))
	}
	if r.WeekStart != nil {
		add("WKST", weekday(*r.WeekStart))
	}
	if r.Skip != 0 {
		add("SKIP", r.Skip.String())
	}
	return strings.Join(parts, ";")
}

// calendar is the scale the rule counts in: Gregorian when it names none.
func (r Rule) calendar() (rscale.Calendar, error) {
	if r.Scale == "" {
		return rscale.Gregorian{}, nil
	}
	c, ok := rscale.Lookup(r.Scale)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrScale, r.Scale)
	}
	return c, nil
}

func (r Rule) validate() error {
	if r.Freq < Secondly || r.Freq > Yearly {
		return syntaxf("no FREQ")
	}
	c, err := r.calendar()
	if err != nil {
		return err
	}
	lim := c.Limits()
	weeks := (lim.YearDays + 6) / 7
	byMonthDayEtc := r.ByMonthDay != nil || r.ByYearDay != nil || r.ByWeekNo != nil
	for _, check := range []struct {
		bad  bool
		what string
	}{
		{r.Until != nil && r.Count != 0, "COUNT with UNTIL"},
		{r.Count < 0, "COUNT below 1"},
		{r.Interval < 0, "INTERVAL below 1"},
		{r.Skip != 0 && r.Scale == "", "SKIP without RSCALE"},
		{r.Skip < 0 || r.Skip > Forward, "unknown SKIP"},
		{r.WeekStart != nil && (*r.WeekStart < time.Sunday || *r.WeekStart > time.Saturday), "unknown WKST"},
		{r.ByWeekNo != nil && r.Freq != Yearly, "BYWEEKNO without FREQ=YEARLY"},
		{r.ByYearDay != nil && (r.Freq == Daily || r.Freq == Weekly || r.Freq == Monthly), "BYYEARDAY with FREQ=" + r.Freq.String()},
		{r.ByMonthDay != nil && r.Freq == Weekly, "BYMONTHDAY with FREQ=WEEKLY"},
		{r.BySetPos != nil && r.BySecond == nil && r.ByMinute == nil && r.ByHour == nil && r.ByDay == nil &&
			!byMonthDayEtc && r.ByMonth == nil, "BYSETPOS alone"},
		{!all(r.BySecond, within(0, 60)), "BYSECOND out of range"},
		{!all(r.ByMinute, within(0, 59)), "BYMINUTE out of range"},
		{!all(r.ByHour, within(0, 23)), "BYHOUR out of range"},
		{!all(r.ByMonthDay, signed(lim.MonthDays)), "BYMONTHDAY out of range"},
		{!all(r.ByYearDay, signed(lim.YearDays)), "BYYEARDAY out of range"},
		{!all(r.ByWeekNo, signed(weeks)), "BYWEEKNO out of range"},
		{!all(r.BySetPos, signed(lim.YearDays)), "BYSETPOS out of range"},
		{!all(r.ByMonth, func(m rscale.Month) bool {
			return m.N >= 1 && m.N <= lim.Months && (!m.Leap || lim.LeapMonths)
		}), "BYMONTH out of range for " + c.Name()},
		{!all(r.ByDay, func(w WeekdayNum) bool {
			return w.Day >= time.Sunday && w.Day <= time.Saturday && (w.N == 0 || signed(weeks)(w.N))
		}), "BYDAY out of range"},
		{!all(r.ByDay, func(w WeekdayNum) bool {
			return w.N == 0 || (r.Freq == Monthly || r.Freq == Yearly) && r.ByWeekNo == nil
		}), "numbered BYDAY outside FREQ=MONTHLY or YEARLY, or with BYWEEKNO"},
	} {
		if check.bad {
			return syntaxf("%s", check.what)
		}
	}
	return nil
}

func syntaxf(format string, a ...any) error {
	return fmt.Errorf("%w: %s", ErrSyntax, fmt.Sprintf(format, a...))
}

func all[T any](values []T, ok func(T) bool) bool {
	for _, v := range values {
		if !ok(v) {
			return false
		}
	}
	return true
}

func within(lo, hi int) func(int) bool {
	return func(n int) bool { return n >= lo && n <= hi }
}

func signed(limit int) func(int) bool {
	return func(n int) bool { return n != 0 && n >= -limit && n <= limit }
}

func join[T any](values []T, format func(T) string) string {
	s := make([]string, len(values))
	for i, v := range values {
		s[i] = format(v)
	}
	return strings.Join(s, ",")
}

func parseList[T any](value string, parse func(string) (T, error)) ([]T, error) {
	var out []T
	for s := range strings.SplitSeq(value, ",") {
		v, err := parse(s)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func parseName[T ~int](value string, name func(T) string, first, last T) (T, error) {
	for v := first; v <= last; v++ {
		if strings.EqualFold(name(v), value) {
			return v, nil
		}
	}
	return 0, fmt.Errorf("unknown %q", value)
}

func parseUnsigned(s string) (int, error) {
	if s == "" || s[0] < '0' || s[0] > '9' {
		return 0, fmt.Errorf("not a number: %q", s)
	}
	return strconv.Atoi(s)
}

func parsePositive(s string) (int, error) {
	n, err := parseUnsigned(s)
	if err == nil && n < 1 {
		return 0, fmt.Errorf("%d is below 1", n)
	}
	return n, err
}

func parseSigned(s string) (int, error) {
	sign, digits := 1, s
	if rest, ok := strings.CutPrefix(s, "-"); ok {
		sign, digits = -1, rest
	} else if rest, ok := strings.CutPrefix(s, "+"); ok {
		digits = rest
	}
	n, err := parseUnsigned(digits)
	return sign * n, err
}

func parseWeekdayNum(s string) (WeekdayNum, error) {
	if len(s) < 2 {
		return WeekdayNum{}, fmt.Errorf("not a weekday: %q", s)
	}
	d, err := parseWeekday(s[len(s)-2:])
	if err != nil || len(s) == 2 {
		return WeekdayNum{0, d}, err
	}
	n, err := parseSigned(s[:len(s)-2])
	if err == nil && n == 0 {
		err = fmt.Errorf("weekday number 0 in %q", s)
	}
	return WeekdayNum{n, d}, err
}

func parseWeekday(s string) (time.Weekday, error) {
	for d := time.Sunday; d <= time.Saturday; d++ {
		if strings.EqualFold(weekday(d), s) {
			return d, nil
		}
	}
	return 0, fmt.Errorf("not a weekday: %q", s)
}

func weekday(d time.Weekday) string {
	return [...]string{"SU", "MO", "TU", "WE", "TH", "FR", "SA"}[d]
}
