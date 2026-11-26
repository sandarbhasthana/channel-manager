package domain

import "math"

// SaveResult says what persisting a reservation did. The OTA pull runs over
// the same bookings on every sync, so callers must tell a new booking from
// one they already have: only a new one is announced to the PMS as created
// (BUG-064: a second Full sync created every OTA booking again).
type SaveResult int

const (
	// SaveInserted: a booking this channel manager had not seen.
	SaveInserted SaveResult = iota
	// SaveUpdated: a known booking whose stay, status or price changed.
	SaveUpdated
	// SaveUnchanged: a known booking, identical to what is stored.
	SaveUnchanged
)

// SameStay reports whether two copies of one booking agree on everything the
// PMS acts on: status, dates (by calendar day) and the amount.
func (r Reservation) SameStay(o Reservation) bool {
	day := "2006-01-02"
	cur := func(c string) string {
		if c == "" {
			return "USD"
		}
		return c
	}
	return r.Status == o.Status &&
		r.CheckIn.Format(day) == o.CheckIn.Format(day) &&
		r.CheckOut.Format(day) == o.CheckOut.Format(day) &&
		math.Round(r.TotalAmount*100) == math.Round(o.TotalAmount*100) &&
		cur(r.Currency) == cur(o.Currency)
}
