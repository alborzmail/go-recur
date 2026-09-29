package rscale

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestGregorianEveryDay(t *testing.T) {
	c := Gregorian{}
	first, _ := c.Year(0)
	last, _ := c.Year(9999)
	n := 0
	for d := first.Start; d < last.End(); d++ {
		tm := time.Unix(int64(d)*86400, 0).UTC()
		want := Date{tm.Year(), Month{N: int(tm.Month())}, tm.Day()}
		if got, ok := DateOf(c, d); !ok || got != want {
			t.Fatalf("DateOf(%d) = %v, %v; want %v", d, got, ok, want)
		}
		if got, ok := DayAt(c, want); !ok || got != d {
			t.Fatalf("DayAt(%v) = %d, %v; want %d", want, got, ok, d)
		}
		if d.Weekday() != tm.Weekday() || DayOf(tm) != d {
			t.Fatalf("day %d: weekday %v, DayOf %d; time says %v", d, d.Weekday(), DayOf(tm), tm.Weekday())
		}
		n++
	}
	if n != 3652425 {
		t.Fatalf("walked %d days", n)
	}
	for _, d := range []Day{first.Start - 1, last.End()} {
		if _, ok := DateOf(c, d); ok {
			t.Errorf("DateOf(%d) outside the years", d)
		}
	}
}

// The golden file comes from alborz's calsys, the code Persian was moved from.
func TestPersianEveryDay(t *testing.T) {
	f, err := os.Open("testdata/persian_years.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	c := Persian{}
	var d Day
	years := 0
	s := bufio.NewScanner(f)
	for s.Scan() {
		if strings.HasPrefix(s.Text(), "#") {
			continue
		}
		fields := strings.Fields(s.Text())
		y, _ := strconv.Atoi(fields[0])
		nowruz, err := time.Parse(time.DateOnly, fields[1])
		if err != nil {
			t.Fatal(err)
		}
		length, _ := strconv.Atoi(fields[2])
		if years > 0 && DayOf(nowruz) != d {
			t.Fatalf("year %d starts on %d; the previous ended before %d", y, DayOf(nowruz), d)
		}
		d = DayOf(nowruz)
		for i := range 12 {
			days := 31 - i/6
			if i == 11 {
				days = length - 336
			}
			for dd := 1; dd <= days; dd++ {
				want := Date{y, Month{N: i + 1}, dd}
				if got, ok := DateOf(c, d); !ok || got != want {
					t.Fatalf("DateOf(%s) = %v, %v; want %v", time.Unix(int64(d)*86400, 0).UTC().Format(time.DateOnly), got, ok, want)
				}
				if got, ok := DayAt(c, want); !ok || got != d {
					t.Fatalf("DayAt(%v) = %d, %v; want %d", want, got, ok, d)
				}
				d++
			}
		}
		years++
	}
	if err := s.Err(); err != nil {
		t.Fatal(err)
	}
	if years != 3239 {
		t.Fatalf("read %d years", years)
	}
	first, _ := c.Year(-61)
	for _, d := range []Day{first.Start - 1, d} {
		if _, ok := DateOf(c, d); ok {
			t.Errorf("DateOf(%d) outside the reckoning", d)
		}
	}
	for _, y := range []int{-62, 3178} {
		if _, ok := c.Year(y); ok {
			t.Errorf("Year(%d) outside the reckoning", y)
		}
	}
}

func TestDayAtRefuses(t *testing.T) {
	for _, date := range []Date{
		{2025, Month{N: 2}, 29},
		{2025, Month{N: 2}, 0},
		{2025, Month{N: 13}, 1},
		{2025, Month{N: 5, Leap: true}, 1},
		{10000, Month{N: 1}, 1},
	} {
		if d, ok := DayAt(Gregorian{}, date); ok {
			t.Errorf("DayAt(%v) = %d", date, d)
		}
	}
	if d, ok := DayAt(Persian{}, Date{1404, Month{N: 12}, 30}); ok {
		t.Errorf("1404 is not a leap year, got day %d", d)
	}
}

func TestMonth(t *testing.T) {
	for s, want := range map[string]Month{"5": {N: 5}, "5L": {5, true}, "5l": {5, true}, "12": {N: 12}} {
		if got, err := ParseMonth(s); err != nil || got != want {
			t.Errorf("ParseMonth(%q) = %v, %v", s, got, err)
		}
	}
	for _, s := range []string{"", "L", "0", "-1", "+1", "1LL", "x"} {
		if m, err := ParseMonth(s); err == nil {
			t.Errorf("ParseMonth(%q) = %v", s, m)
		}
	}
	if s := (Month{5, true}).String(); s != "5L" {
		t.Errorf("String = %q", s)
	}
	if !(Month{N: 5}).Before(Month{5, true}) || !(Month{5, true}).Before(Month{N: 6}) || (Month{N: 6}).Before(Month{5, true}) {
		t.Error("a leap month does not follow its number")
	}
}

func TestLookup(t *testing.T) {
	for _, name := range []string{"GREGORIAN", "gregorian", "Persian"} {
		if c, ok := Lookup(name); !ok || !strings.EqualFold(c.Name(), name) {
			t.Errorf("Lookup(%q) = %v, %v", name, c, ok)
		}
	}
	if _, ok := Lookup("HEBREW"); ok {
		t.Error("Lookup(HEBREW) found a calendar this module does not ship")
	}
}

func TestDayOfZone(t *testing.T) {
	tehran, err := time.LoadLocation("Asia/Tehran")
	if err != nil {
		t.Fatal(err)
	}
	tm := time.Date(2025, 3, 20, 23, 30, 0, 0, tehran)
	if got, want := DayOf(tm), DayOf(time.Date(2025, 3, 20, 0, 0, 0, 0, time.UTC)); got != want {
		t.Errorf("DayOf(%s) = %d; want the wall date %d", tm, got, want)
	}
}
