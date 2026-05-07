package domain

import "time"

// InventoryDay represents availability and restrictions for a single room type on a single date.
// Fields map 1-to-1 with inventory.inventory_days columns; PK is (OrgID, RoomTypeID, StayDate).
type InventoryDay struct {
	OrgID      string    `json:"org_id"`
	RoomTypeID string    `json:"room_type_id"`
	StayDate   time.Time `json:"stay_date"`
	Available  int       `json:"available"`
	Sold       int       `json:"sold"`
	Blocked    int       `json:"blocked"`
	StopSell   bool      `json:"stop_sell"`
	MinStay    *int32    `json:"min_stay,omitempty"` // nil = no restriction
	MaxStay    *int32    `json:"max_stay,omitempty"` // nil = no restriction
	CTA        bool      `json:"cta"`                // closed to arrival
	CTD        bool      `json:"ctd"`                // closed to departure
	Version    int64     `json:"version"`
	UpdatedAt  time.Time `json:"updated_at"`
}
