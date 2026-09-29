package recur

import "time"

// Kind is the type of an iCalendar time value.
type Kind int

const (
	DateTime Kind = iota // with a zone, or UTC
	Floating
	Date
)

// Value is an iCalendar DATE or DATE-TIME. A floating time or a date is
// read by its wall clock in the location of the set it belongs to.
type Value struct {
	time.Time
	Kind Kind
}

// ParseValue reads a DATE, a floating DATE-TIME or a UTC DATE-TIME.
func ParseValue(s string) (Value, error) {
	kind, layout := Date, dateLayout
	switch len(s) {
	case len(floatingLayout):
		kind, layout = Floating, floatingLayout
	case len(utcLayout):
		kind, layout = DateTime, utcLayout
	}
	t, err := time.Parse(layout, s)
	return Value{t, kind}, err
}

func (v Value) String() string {
	switch v.Kind {
	case Date:
		return v.Time.Format(dateLayout)
	case Floating:
		return v.Time.Format(floatingLayout)
	}
	return v.Time.UTC().Format(utcLayout)
}

const (
	dateLayout     = "20060102"
	floatingLayout = "20060102T150405"
	utcLayout      = "20060102T150405Z"
)

// Period is an RDATE: a start and, for VALUE=PERIOD, its end; the zero
// End for a plain date or date-time.
type Period struct {
	Start Value
	End   Value
}
