package calendar

import (
	"time"
)

func GetLagosLocation() *time.Location {
	loc, err := time.LoadLocation("Africa/Lagos")
	if err != nil {
		// Fallback to WAT (UTC+1)
		loc = time.FixedZone("Africa/Lagos", 3600)
	}
	return loc
}

// CalculateLastSaturdayOfLagos calculates the last Saturday of the specified year and month
// in the Africa/Lagos timezone at 07:00:00 WAT.
func CalculateLastSaturdayOfLagos(year, month int) time.Time {
	loc := GetLagosLocation()

	// Start from the first day of the next month
	firstOfNextMonth := time.Date(year, time.Month(month+1), 1, 0, 0, 0, 0, loc)
	// Last day of target month
	lastDay := firstOfNextMonth.AddDate(0, 0, -1)

	// Step backwards until Saturday
	for lastDay.Weekday() != time.Saturday {
		lastDay = lastDay.AddDate(0, 0, -1)
	}

	return time.Date(lastDay.Year(), lastDay.Month(), lastDay.Day(), 7, 0, 0, 0, loc)
}
