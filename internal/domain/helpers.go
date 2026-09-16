package domain

import "time"

// TimeToMillis convertit un time.Time en millisecondes Unix.
func TimeToMillis(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixMilli()
}

// MillisToTime convertit un timestamp millisecondes Unix en time.Time UTC.
func MillisToTime(ms int64) time.Time {
	if ms <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(ms).UTC()
}
