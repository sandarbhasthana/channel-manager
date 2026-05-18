package domain

import "time"

// ChannelCapability represents a feature supported by a channel adapter.
type ChannelCapability string

const (
	CapabilityPushAvailability  ChannelCapability = "push_availability"
	CapabilityPushRates         ChannelCapability = "push_rates"
	CapabilityFetchReservations ChannelCapability = "fetch_reservations"
	CapabilityPushReservations  ChannelCapability = "push_reservations"
)

// AvailabilityUpdate represents an availability push payload.
type AvailabilityUpdate struct {
	PropertyID string    `json:"property_id"`
	RoomTypeID string    `json:"room_type_id"`
	Date       time.Time `json:"date"`
	Available  int       `json:"available"`
	StopSell   bool      `json:"stop_sell"`
	MinStay    int       `json:"min_stay"`
	MaxStay    int       `json:"max_stay"`
}

// RateUpdate represents a rate push payload.
type RateUpdate struct {
	PropertyID string    `json:"property_id"`
	RoomTypeID string    `json:"room_type_id"`
	RatePlanID string    `json:"rate_plan_id"`
	Date       time.Time `json:"date"`
	BaseRate   float64   `json:"base_rate"`
	Currency   string    `json:"currency"`
}

// FetchedReservation represents a reservation fetched from a channel.
type FetchedReservation struct {
	ChannelConfirmationID string    `json:"channel_confirmation_id"`
	GuestName             string    `json:"guest_name"`
	RoomTypeExternalID    string    `json:"room_type_external_id"`
	CheckIn               time.Time `json:"check_in"`
	CheckOut              time.Time `json:"check_out"`
	Status                string    `json:"status"`
	TotalAmount           float64   `json:"total_amount"`
	Currency              string    `json:"currency"`
}

// Connection represents an org-level credential/account for an OTA provider.
type Connection struct {
	ID         string        `json:"id"`
	OrgID      string        `json:"org_id"`
	Provider   string        `json:"provider"` // e.g. "airbnb"
	Name       string        `json:"name"`
	Status     string        `json:"status"` // inactive, active, error, disabled
	SecretRef  string        `json:"secret_ref"`
	Config     MappingConfig `json:"config"`
	LastSyncAt *time.Time    `json:"last_sync_at"`
	LastError  string        `json:"last_error"`
	CreatedAt  time.Time     `json:"created_at"`
	UpdatedAt  time.Time     `json:"updated_at"`
}

// Channel represents a property-level OTA listing attached to an org-level Connection.
type Channel struct {
	ID                 string     `json:"id"`
	OrgID              string     `json:"org_id"`
	PropertyID         string     `json:"property_id"`
	ConnectionID       string     `json:"connection_id"`
	Provider           string     `json:"provider"`
	ExternalPropertyID string     `json:"external_property_id"`
	Status             string     `json:"status"` // inactive, active, paused, error, disconnected
	LastSyncAt         *time.Time `json:"last_sync_at"`
	LastError          string     `json:"last_error"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

// MappingConfig stores per-connection mapping data.
type MappingConfig struct {
	PropertyMap  map[string]string `json:"property_map"`
	RoomMap      map[string]string `json:"room_map"`
	RatePlanMap  map[string]string `json:"rate_plan_map"`
	Currency     string            `json:"currency"`
	Timezone     string            `json:"timezone"`
	PollInterval int               `json:"poll_interval_seconds"`
}

// SyncJobType defines the type of work in a sync job.
type SyncJobType string

const (
	JobTypeInventoryPush   SyncJobType = "inventory_push"
	JobTypePricingPush     SyncJobType = "pricing_push"
	JobTypeReservationPull SyncJobType = "reservation_pull"
	JobTypeMappingSync     SyncJobType = "mapping_sync"
	JobTypeFullSync        SyncJobType = "full_sync"
)

// SyncJobStatus defines the lifecycle state of a sync job.
type SyncJobStatus string

const (
	StatusQueued    SyncJobStatus = "queued"
	StatusRunning   SyncJobStatus = "running"
	StatusSucceeded SyncJobStatus = "succeeded"
	StatusFailed    SyncJobStatus = "failed"
	StatusCancelled SyncJobStatus = "cancelled"
)

// SyncJob represents an outbound work unit for an OTA adapter.
type SyncJob struct {
	ID           string        `json:"id"`
	OrgID        string        `json:"org_id"`
	ConnectionID string        `json:"connection_id"`
	JobType      SyncJobType   `json:"job_type"`
	Status       SyncJobStatus `json:"status"`
	Payload      any           `json:"payload"`
	Result       any           `json:"result"`
	Attempts     int           `json:"attempts"`
	LastError    string        `json:"last_error"`
	ScheduledAt  time.Time     `json:"scheduled_at"`
	StartedAt    *time.Time    `json:"started_at"`
	FinishedAt   *time.Time    `json:"finished_at"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
}
