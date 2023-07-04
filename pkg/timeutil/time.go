package timeutil

import "time"

func InSpan(t, from, to time.Time) bool {
	return t.Equal(from) || t.Equal(to) || t.After(from) && t.Before(to)
}
