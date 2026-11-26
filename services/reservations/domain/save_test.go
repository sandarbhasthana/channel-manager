package domain

import (
	"testing"
	"time"
)

func TestSameStay(t *testing.T) {
	base := Reservation{
		Status:      "confirmed",
		CheckIn:     time.Date(2026, 12, 10, 0, 0, 0, 0, time.UTC),
		CheckOut:    time.Date(2026, 12, 12, 0, 0, 0, 0, time.UTC),
		TotalAmount: 250,
		Currency:    "USD",
	}
	same := base
	// The stored copy comes back from Postgres with a zone and float noise.
	same.CheckIn = base.CheckIn.In(time.FixedZone("x", 0))
	same.TotalAmount = 250.0000001
	same.Currency = ""
	if !base.SameStay(same) {
		t.Fatal("identical booking reported as changed")
	}
	for name, mut := range map[string]func(*Reservation){
		"status":    func(r *Reservation) { r.Status = "cancelled" },
		"check-in":  func(r *Reservation) { r.CheckIn = r.CheckIn.AddDate(0, 0, 1) },
		"check-out": func(r *Reservation) { r.CheckOut = r.CheckOut.AddDate(0, 0, 1) },
		"amount":    func(r *Reservation) { r.TotalAmount = 300 },
		"currency":  func(r *Reservation) { r.Currency = "EUR" },
	} {
		c := base
		mut(&c)
		if base.SameStay(c) {
			t.Errorf("%s change not detected", name)
		}
	}
}
