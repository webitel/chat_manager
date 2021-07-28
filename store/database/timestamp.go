package database

import (
	"time"
)

var (
	
	// CurrentTime returns the current local time.
	CurrentTime = time.Now

	// TimeStamp represents default human-readable timestamp layout format
	TimeStamp = "2006-01-02 15:04:05.000"
	// Precision as a default value for *Timestamp* -like functions
	TimePrecision = time.Millisecond // 1e6

	// unixEpoch date time constant
	unixEpoch = time.Date(1970, time.January, 01, 00, 00, 00, 000000000, time.UTC)
)

// Date2Epochtime returns number of seconds,
// posibly precised with app.TimePrecision,
// elapsed since January 1, 1970 UTC
// until given date time
func DateEpochtime(date time.Time, precision time.Duration) (nsec int64) {

	if date.IsZero() || date.Before(unixEpoch) {
		return 0
	}

	switch precision {

	case time.Second:
		return date.Unix() // seconds

	case time.Millisecond,
		 time.Microsecond:
		return date.UnixNano()/(int64)(precision)

	case time.Nanosecond:
		return date.UnixNano()

	default:
		panic("epochtime: invalid precision "+ precision.String())
	}

	panic("unreachable code")
}

func EpochtimeDate(nsec int64, precision time.Duration) (date time.Time) {

	if nsec == 0 {
		return date // date.IsZero(!)
	}

	switch precision {

	case time.Second:
		return time.Unix(nsec, 0) // seconds

	case time.Millisecond,
		 time.Microsecond,
		 time.Nanosecond:

		epochToTimestamp := (int64)(time.Second/precision)
		
		return time.Unix(
			nsec/epochToTimestamp,
			nsec%epochToTimestamp*(int64)(precision),
		)

	default:
		panic("epochtime: invalid precision "+ precision.String())
	}

	panic("unreachable code")
}

func DateTimestamp(date time.Time) (nsec int64) {
	return DateEpochtime(date, TimePrecision)
}

func TimestampDate(tsec int64) (date time.Time) {
	return EpochtimeDate(tsec, TimePrecision)
}