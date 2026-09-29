// Package rscale counts days in the calendars a recurrence rule may name
// with RSCALE (RFC 7529).
package rscale

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Day counts days from 1970-01-01 in the proleptic Gregorian calendar.
type Day int

// DayOf is the wall date of t, in t's location.
func DayOf(t time.Time) Day {
	_, offset := t.Zone()
	return Day(floorDiv(t.Unix()+int64(offset), 86400))
}

func (d Day) Weekday() time.Weekday {
	return time.Weekday((int(d)%7 + 11) % 7)
}

// Month is a month of a year: its number, and whether it is the leap
// month that follows that number (RFC 7529 4.2, "5L").
type Month struct {
	N    int
	Leap bool
}

func ParseMonth(s string) (Month, error) {
	digits, leap := strings.CutSuffix(strings.ToUpper(s), "L")
	n, err := strconv.Atoi(digits)
	if err != nil || n < 1 || digits[0] == '+' {
		return Month{}, fmt.Errorf("not a month: %q", s)
	}
	return Month{n, leap}, nil
}

func (m Month) String() string {
	if m.Leap {
		return strconv.Itoa(m.N) + "L"
	}
	return strconv.Itoa(m.N)
}

// Before orders months within a year: a leap month follows its number.
func (m Month) Before(o Month) bool {
	return m.N < o.N || m.N == o.N && !m.Leap && o.Leap
}

// Year is one year of a calendar.
type Year struct {
	Start  Day
	Months []Span // in order
}

type Span struct {
	Month Month
	Days  int
}

// End is the first day of the next year.
func (y Year) End() Day {
	end := y.Start
	for _, s := range y.Months {
		end += Day(s.Days)
	}
	return end
}

// Limits bound a rule's BY values in a calendar (RFC 7529 4, item 1).
type Limits struct {
	Months     int  // largest regular month number
	LeapMonths bool // whether "nL" is meaningful
	MonthDays  int  // longest month
	YearDays   int  // longest year
}

// Calendar counts days in years and months.
type Calendar interface {
	// Name is the RSCALE name, upper case (CLDR: GREGORIAN, PERSIAN).
	Name() string
	// Year returns year y; false outside the years the calendar reckons.
	Year(y int) (Year, bool)
	// YearOf returns the year d falls in; false outside the reckoning.
	YearOf(d Day) (int, bool)
	Limits() Limits
}

// Lookup returns the calendar an RSCALE names, case-insensitively.
func Lookup(rscale string) (Calendar, bool) {
	for _, c := range []Calendar{Gregorian{}, Persian{}} {
		if strings.EqualFold(c.Name(), rscale) {
			return c, true
		}
	}
	return nil, false
}

// Date is a day as a calendar names it.
type Date struct {
	Year  int
	Month Month
	Day   int
}

func DateOf(c Calendar, d Day) (Date, bool) {
	y, ok := c.YearOf(d)
	if !ok {
		return Date{}, false
	}
	year, _ := c.Year(y)
	k := int(d - year.Start)
	for _, s := range year.Months {
		if k < s.Days {
			return Date{y, s.Month, k + 1}, true
		}
		k -= s.Days
	}
	panic(fmt.Sprintf("rscale: %s places day %d in year %d, which does not hold it", c.Name(), d, y))
}

// DayAt is the day a date names; false for a date c does not have.
func DayAt(c Calendar, date Date) (Day, bool) {
	year, ok := c.Year(date.Year)
	if !ok {
		return 0, false
	}
	start := year.Start
	for _, s := range year.Months {
		if s.Month == date.Month {
			return start + Day(date.Day-1), date.Day >= 1 && date.Day <= s.Days
		}
		start += Day(s.Days)
	}
	return 0, false
}

func floorDiv(a, b int64) int64 {
	q := a / b
	if a%b < 0 {
		q--
	}
	return q
}
