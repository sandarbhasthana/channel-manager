package domain

import "time"

// RateDay represents pricing for a single room type and rate plan on a single date.
type RateDay struct {
	ID           string             `json:"id"`
	OrgID        string             `json:"org_id"`
	PropertyID   string             `json:"property_id"`
	RoomTypeID   string             `json:"room_type_id"`
	RatePlanID   string             `json:"rate_plan_id"`
	Date         time.Time          `json:"date"`
	BaseRate     float64            `json:"base_rate"`
	Currency     string             `json:"currency"`
	DerivedRates map[string]float64 `json:"derived_rates,omitempty"`
}
