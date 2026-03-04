package util

import "time"

const TimestampLayout = "15:04:05 02-01-2006"

func NowTimestamp() string {
	return time.Now().Format(TimestampLayout)
}

func ParseTimestamp(s string) (time.Time, error) {
	return time.ParseInLocation(TimestampLayout, s, time.Local)
}
