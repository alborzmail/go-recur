package rscale

// Persian is the Solar Hijri calendar, reckoned by Borkowski's
// algorithm (Earth, Moon and Planets 74, 1996) over the years it holds
// for, -61 to 3177.
type Persian struct{}

const persianFirst, persianLast = -61, 3177

func (Persian) Name() string { return "PERSIAN" }

func (Persian) Limits() Limits { return Limits{Months: 12, MonthDays: 31, YearDays: 366} }

// The first six months hold 31 days, the next five 30, and Esfand 29,
// or 30 in a leap year.
func (Persian) Year(y int) (Year, bool) {
	if y < persianFirst || y > persianLast {
		return Year{}, false
	}
	since, nowruz := shYear(y)
	months := make([]Span, 12)
	for i := range months {
		months[i] = Span{Month{N: i + 1}, 31 - i/6}
	}
	if since != 0 {
		months[11].Days = 29
	}
	return Year{nowruz, months}, true
}

func (p Persian) YearOf(d Day) (int, bool) {
	gy, _, _ := civil(d)
	for _, y := range []int{gy - 621, gy - 622} {
		if year, ok := p.Year(y); ok && d >= year.Start {
			return y, d < year.End()
		}
	}
	return 0, false
}

// shYear returns the years since the last leap year (0 for a leap year)
// and the day of Nowruz.
func shYear(jy int) (sinceLeap int, nowruz Day) {
	// The years the reckoning restarts from; the last is the first it cannot count.
	breaks := [...]int{-61, 9, 38, 199, 426, 686, 756, 818, 1111, 1181, 1210, 1635, 2060, 2097, 2192, 2262, 2324, 2394, 2456, 3178}
	gy := jy + 621
	leapJ, jp, jump := -14, breaks[0], 0
	for _, jm := range breaks[1:] {
		jump = jm - jp
		if jy < jm {
			break
		}
		leapJ += jump/33*8 + jump%33/4
		jp = jm
	}
	n := jy - jp
	leapJ += n/33*8 + (n%33+3)/4
	if jump%33 == 4 && jump-n == 4 {
		leapJ++
	}
	leapG := gy/4 - (gy/100+1)*3/4 - 150
	march := 20 + leapJ - leapG
	if jump-n < 6 {
		n = n - jump + (jump+4)/33*33
	}
	since := ((n+1)%33 - 1) % 4
	if since == -1 {
		since = 4
	}
	return since, civilDay(gy, 3, march)
}
