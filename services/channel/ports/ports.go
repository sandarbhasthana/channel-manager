package ports

import (
	"context"
	"time"

	"github.com/channel-manager/channel-manager/services/channel/domain"
)

// ChannelAdapter is the primary interface every OTA/channel adapter must implement.
type ChannelAdapter interface {
	// ChannelID returns the unique identifier for this channel.
	ChannelID() string

	// Capabilities returns the set of features this adapter supports.
	Capabilities() []domain.ChannelCapability
}

// AvailabilityPusher pushes availability updates to a channel.
type AvailabilityPusher interface {
	PushAvailability(ctx context.Context, updates []domain.AvailabilityUpdate) error
}

// RatePusher pushes rate updates to a channel.
type RatePusher interface {
	PushRates(ctx context.Context, updates []domain.RateUpdate) error
}

// ReservationFetcher fetches reservations from a channel.
type ReservationFetcher interface {
	FetchReservations(ctx context.Context, propertyID string, since time.Time) ([]domain.FetchedReservation, error)
}

// ConnectionRepository provides persistence for org-level OTA connections.
type ConnectionRepository interface {
	Create(ctx context.Context, conn domain.Connection) (domain.Connection, error)
	GetByID(ctx context.Context, id string) (domain.Connection, error)
	ListByOrg(ctx context.Context, orgID string) ([]domain.Connection, error)
	UpdateStatus(ctx context.Context, id string, status string, lastError string) error
	UpdateName(ctx context.Context, id string, name string) error
	UpdateLastSync(ctx context.Context, id string, lastSyncAt time.Time) error
	Delete(ctx context.Context, id string) error
}

// ChannelRepository provides persistence for property-level OTA channels.
type ChannelRepository interface {
	Create(ctx context.Context, ch domain.Channel) (domain.Channel, error)
	GetByID(ctx context.Context, id string) (domain.Channel, error)
	ListByProperty(ctx context.Context, propertyID string) ([]domain.Channel, error)
	UpdateStatus(ctx context.Context, id string, status string, lastError string) error
	UpdateLastSync(ctx context.Context, id string, lastSyncAt time.Time) error
	Delete(ctx context.Context, id string) error
}

// SyncJobRepository provides persistence for sync jobs.
type SyncJobRepository interface {
	Create(ctx context.Context, job domain.SyncJob) error
	GetByID(ctx context.Context, id string) (domain.SyncJob, error)
	UpdateStatus(ctx context.Context, id string, status domain.SyncJobStatus, result any, lastError string) error
	ListRecentByConnection(ctx context.Context, connectionID string, limit int32) ([]domain.SyncJob, error)
}

// SecretResolver stores and resolves secret references to live credentials.
type SecretResolver interface {
	// Store persists credentials and returns an opaque secret_ref string.
	Store(ctx context.Context, ref string, creds map[string]string) (string, error)
	// Resolve retrieves live credentials from a secret_ref.
	Resolve(ctx context.Context, ref string) (map[string]string, error)
}

// ChannelEventPublisher publishes domain events for channel operations.
type ChannelEventPublisher interface {
	PublishSyncSucceeded(ctx context.Context, job domain.SyncJob) error
	PublishSyncFailed(ctx context.Context, job domain.SyncJob) error
}

// CircuitBreaker provides protection against failing outbound calls.
type CircuitBreaker interface {
	Allow(key string) (func(success bool), error)
}
