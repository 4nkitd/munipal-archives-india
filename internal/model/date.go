package model

import (
	"strings"
	"time"
)

func PublicationDay(date string) string {
	date = strings.TrimSpace(date)
	if len(date) == len(time.DateOnly) {
		if _, err := time.Parse(time.DateOnly, date); err == nil {
			return date
		}
	}
	return ""
}
