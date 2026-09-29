package rscale

// Gregorian is the proleptic Gregorian calendar over the years an
// iCalendar date can write, 0000 to 9999.
type Gregorian struct{}

const gregorianFirst, gregorianLast = 0, 9999

func (Gregorian) Name() string { return "GREGORIAN" }

func (Gregorian) Limits() Limits { return Limits{Months: 12, MonthDays: 31, YearDays: 366} }

func (Gregorian) Year(y int) (Year, bool) {
	if y < gregorianFirst || y > gregorianLast {
		return Year{}, false
	}
	feb := 28
	if y%4 == 0 && (y%100 != 0 || y%400 == 0) {
		feb = 29
	}
	months := make([]Span, 12)
	for i, days := range [12]int{31, feb, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31} {
		months[i] = Span{Month{N: i + 1}, days}
	}
	return Year{civilDay(y, 1, 1), months}, true
}

func (Gregorian) YearOf(d Day) (int, bool) {
	y, _, _ := civil(d)
	return y, y >= gregorianFirst && y <= gregorianLast
}

// civilDay and civil are Howard Hinnant's days_from_civil and civil_from_days.
func civilDay(y, m, d int) Day {
	if m <= 2 {
		y--
	}
	era := floorDiv(int64(y), 400)
	yoe := y - int(era)*400
	mp := (m + 9) % 12
	doy := (153*mp+2)/5 + d - 1
	doe := yoe*365 + yoe/4 - yoe/100 + doy
	return Day(int(era)*146097 + doe - 719468)
}

func civil(d Day) (y, m, dd int) {
	z := int64(d) + 719468
	era := floorDiv(z, 146097)
	doe := int(z - era*146097)
	yoe := (doe - doe/1460 + doe/36524 - doe/146096) / 365
	doy := doe - (365*yoe + yoe/4 - yoe/100)
	mp := (5*doy + 2) / 153
	dd = doy - (153*mp+2)/5 + 1
	m = (mp+2)%12 + 1
	y = yoe + int(era)*400
	if m <= 2 {
		y++
	}
	return y, m, dd
}
