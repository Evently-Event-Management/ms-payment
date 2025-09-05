package utils

import (
	"time"
)

// UnixTimeToTime converts a Unix timestamp to a time.Time object
func UnixTimeToTime(unixTime int64) time.Time {
	return time.Unix(unixTime, 0)
}
