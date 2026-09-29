# go-recur

iCalendar recurrence for Go with RFC 7529 calendar scales.

- RRULE, RDATE (including periods), EXDATE and overridden instances
  with RANGE=THISANDFUTURE (RFC 5545)
- RSCALE and SKIP (RFC 7529), Gregorian and Persian (Solar Hijri)
- strict parsing; iteration with `iter.Seq`
- every rule ends where its calendar does, however barren

```go
rule, err := recur.Parse("RSCALE=PERSIAN;FREQ=YEARLY;SKIP=BACKWARD")
set := recur.Set{Start: recur.Value{Time: start}, Rule: &rule}
seq, err := set.Between(from, to)
for t := range seq {
	fmt.Println(t)
}
```

The vectors in `testdata` come from libical, python-dateutil and
rrule-go under their own licences, noted in each file. The code is MIT.
