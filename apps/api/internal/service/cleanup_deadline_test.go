package service

import (
	"testing"
	"time"
)

func TestCleanupHasEnded(t *testing.T) {
	end := time.Date(2026, 11, 28, 10, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name, status string
		now          time.Time
		want         bool
	}{
		{"before", "published", end.Add(-time.Nanosecond), false},
		{"at deadline", "published", end, true},
		{"after", "published", end.Add(time.Second), true},
		{"draft stays private", "draft", end.Add(time.Hour), false},
		{"cancelled stays cancelled", "cancelled", end.Add(time.Hour), false},
		{"completed", "completed", end.Add(-time.Hour), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := CleanupHasEnded(tc.status, end, tc.now); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}
