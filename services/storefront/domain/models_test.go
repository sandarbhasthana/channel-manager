package domain

import (
	"testing"
	"time"
)

func day(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

// Stays are half-open [checkin, checkout): a guest checking out on the 5th does
// not conflict with a guest checking in on the 5th.
func TestHoldOverlaps(t *testing.T) {
	hold := Hold{Checkin: day("2026-07-03"), Checkout: day("2026-07-05")}

	cases := []struct {
		name              string
		checkin, checkout string
		want              bool
	}{
		{"identical range", "2026-07-03", "2026-07-05", true},
		{"fully inside", "2026-07-03", "2026-07-04", true},
		{"straddles start", "2026-07-01", "2026-07-04", true},
		{"straddles end", "2026-07-04", "2026-07-07", true},
		{"encloses hold", "2026-07-01", "2026-07-09", true},
		{"checkout on hold checkin", "2026-07-01", "2026-07-03", false},
		{"checkin on hold checkout", "2026-07-05", "2026-07-07", false},
		{"entirely before", "2026-06-01", "2026-06-03", false},
		{"entirely after", "2026-08-01", "2026-08-03", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := hold.Overlaps(day(tc.checkin), day(tc.checkout))
			if got != tc.want {
				t.Errorf("Overlaps(%s, %s) = %v, want %v", tc.checkin, tc.checkout, got, tc.want)
			}
		})
	}
}
