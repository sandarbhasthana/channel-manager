package usecases

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/channel-manager/channel-manager/services/channel/domain"
	"github.com/channel-manager/channel-manager/services/channel/ports"
)

// ChannelService orchestrates channel adapter operations and provides CRUD
// for both org-level Connections and property-level Channels.
type ChannelService struct {
	adapters  map[string]ports.ChannelAdapter
	connRepo  ports.ConnectionRepository
	chanRepo  ports.ChannelRepository
	jobs      ports.SyncJobRepository
	secrets   ports.SecretResolver
	publisher ports.ChannelEventPublisher
	breaker   ports.CircuitBreaker
	log       *slog.Logger
}

// NewChannelService creates a new ChannelService.
func NewChannelService(
	connRepo ports.ConnectionRepository,
	chanRepo ports.ChannelRepository,
	jobs ports.SyncJobRepository,
	secrets ports.SecretResolver,
	publisher ports.ChannelEventPublisher,
	breaker ports.CircuitBreaker,
) *ChannelService {
	return &ChannelService{
		adapters:  make(map[string]ports.ChannelAdapter),
		connRepo:  connRepo,
		chanRepo:  chanRepo,
		jobs:      jobs,
		secrets:   secrets,
		publisher: publisher,
		breaker:   breaker,
		log:       slog.Default().With("service", "channel"),
	}
}

// RegisterAdapter registers a channel adapter.
func (s *ChannelService) RegisterAdapter(adapter ports.ChannelAdapter) {
	s.adapters[adapter.ChannelID()] = adapter
}

// ---------------------------------------------------------------------------
// Connection CRUD (org-level)
// ---------------------------------------------------------------------------

// CreateConnection creates a new org-level OTA connection.
// If credentials are provided they are stored via the SecretResolver and the
// resulting secret_ref is persisted on the connection row.
func (s *ChannelService) CreateConnection(ctx context.Context, conn domain.Connection, credentials map[string]string) (domain.Connection, error) {
	conn.ID = uuid.NewString()
	if conn.Status == "" {
		conn.Status = "active"
	}

	// Store credentials if provided.
	if len(credentials) > 0 && s.secrets != nil {
		ref, err := s.secrets.Store(ctx, "", credentials)
		if err != nil {
			return domain.Connection{}, fmt.Errorf("channel: store credentials: %w", err)
		}
		conn.SecretRef = ref
	}

	return s.connRepo.Create(ctx, conn)
}

// GetConnection returns an org-level connection by ID.
func (s *ChannelService) GetConnection(ctx context.Context, id string) (domain.Connection, error) {
	return s.connRepo.GetByID(ctx, id)
}

// ListConnections returns all connections for the org resolved from ctx.
func (s *ChannelService) ListConnections(ctx context.Context, orgID string) ([]domain.Connection, error) {
	return s.connRepo.ListByOrg(ctx, orgID)
}

// UpdateConnection updates mutable fields on an org-level connection.
// If credentials are provided they are stored/rotated via the SecretResolver.
// status="" means "do not update status".
func (s *ChannelService) UpdateConnection(ctx context.Context, id string, name string, credentials map[string]string, status string) error {
	// Rotate credentials if provided.
	if len(credentials) > 0 && s.secrets != nil {
		conn, err := s.connRepo.GetByID(ctx, id)
		if err != nil {
			return fmt.Errorf("channel: get connection for cred update: %w", err)
		}
		ref, err := s.secrets.Store(ctx, conn.SecretRef, credentials)
		if err != nil {
			return fmt.Errorf("channel: rotate credentials: %w", err)
		}
		// If the ref changed (new connection had no secret_ref), update it.
		if ref != conn.SecretRef {
			s.log.Info("secret_ref updated", "connection_id", id, "old_ref", conn.SecretRef, "new_ref", ref)
		}
	}

	if status != "" {
		if err := s.connRepo.UpdateStatus(ctx, id, status, ""); err != nil {
			return fmt.Errorf("channel: update connection status: %w", err)
		}
	}
	if name != "" {
		return s.connRepo.UpdateName(ctx, id, name)
	}
	return nil
}

// DeleteConnection deletes an org-level connection (cascades to channels).
func (s *ChannelService) DeleteConnection(ctx context.Context, id string) error {
	return s.connRepo.Delete(ctx, id)
}

// ---------------------------------------------------------------------------
// Channel CRUD (property-level)
// ---------------------------------------------------------------------------

// ConnectChannel links a property to an existing org-level Connection.
func (s *ChannelService) ConnectChannel(ctx context.Context, ch domain.Channel) (domain.Channel, error) {
	// Verify the connection exists.
	conn, err := s.connRepo.GetByID(ctx, ch.ConnectionID)
	if err != nil {
		return domain.Channel{}, fmt.Errorf("channel: connection not found: %w", err)
	}

	ch.ID = uuid.NewString()
	ch.Provider = conn.Provider
	if ch.Status == "" {
		ch.Status = "active"
	}
	return s.chanRepo.Create(ctx, ch)
}

// GetChannel returns a channel by ID.
func (s *ChannelService) GetChannel(ctx context.Context, id string) (domain.Channel, error) {
	return s.chanRepo.GetByID(ctx, id)
}

// ListChannels returns all channels for a property.
func (s *ChannelService) ListChannels(ctx context.Context, propertyID string) ([]domain.Channel, error) {
	return s.chanRepo.ListByProperty(ctx, propertyID)
}

// PauseChannel sets a channel's status to paused.
func (s *ChannelService) PauseChannel(ctx context.Context, id string) (domain.Channel, error) {
	if err := s.chanRepo.UpdateStatus(ctx, id, "paused", ""); err != nil {
		return domain.Channel{}, err
	}
	return s.chanRepo.GetByID(ctx, id)
}

// ResumeChannel sets a channel's status to active.
func (s *ChannelService) ResumeChannel(ctx context.Context, id string) (domain.Channel, error) {
	if err := s.chanRepo.UpdateStatus(ctx, id, "active", ""); err != nil {
		return domain.Channel{}, err
	}
	return s.chanRepo.GetByID(ctx, id)
}

// DisconnectChannel deletes a channel.
func (s *ChannelService) DisconnectChannel(ctx context.Context, id string) (domain.Channel, error) {
	ch, err := s.chanRepo.GetByID(ctx, id)
	if err != nil {
		return domain.Channel{}, err
	}
	if err := s.chanRepo.Delete(ctx, id); err != nil {
		return domain.Channel{}, err
	}
	ch.Status = "disconnected"
	return ch, nil
}

// ---------------------------------------------------------------------------
// Sync / data-plane operations (called by the sync worker)
// ---------------------------------------------------------------------------

// PushAvailability dispatches an availability update to the appropriate channel.
func (s *ChannelService) PushAvailability(ctx context.Context, connectionID string, updates []domain.AvailabilityUpdate) error {
	return s.dispatch(ctx, connectionID, domain.JobTypeInventoryPush, domain.CapabilityPushAvailability, func(adapter ports.ChannelAdapter) error {
		pusher, ok := adapter.(ports.AvailabilityPusher)
		if !ok {
			return domain.ErrNotImplemented
		}
		return pusher.PushAvailability(ctx, updates)
	}, updates)
}

// PushRates dispatches a rate update to the appropriate channel.
func (s *ChannelService) PushRates(ctx context.Context, connectionID string, updates []domain.RateUpdate) error {
	return s.dispatch(ctx, connectionID, domain.JobTypePricingPush, domain.CapabilityPushRates, func(adapter ports.ChannelAdapter) error {
		pusher, ok := adapter.(ports.RatePusher)
		if !ok {
			return domain.ErrNotImplemented
		}
		return pusher.PushRates(ctx, updates)
	}, updates)
}

// FetchReservations pulls reservations from a channel.
func (s *ChannelService) FetchReservations(ctx context.Context, connectionID string, propertyID string, since time.Time) ([]domain.FetchedReservation, error) {
	var results []domain.FetchedReservation
	err := s.dispatch(ctx, connectionID, domain.JobTypeReservationPull, domain.CapabilityFetchReservations, func(adapter ports.ChannelAdapter) error {
		fetcher, ok := adapter.(ports.ReservationFetcher)
		if !ok {
			return domain.ErrNotImplemented
		}
		var err error
		results, err = fetcher.FetchReservations(ctx, propertyID, since)
		return err
	}, propertyID)
	return results, err
}

// dispatch handles the common orchestration logic for all channel operations.
func (s *ChannelService) dispatch(
	ctx context.Context,
	connectionID string,
	jobType domain.SyncJobType,
	capability domain.ChannelCapability,
	fn func(ports.ChannelAdapter) error,
	payload any,
) error {
	conn, err := s.connRepo.GetByID(ctx, connectionID)
	if err != nil {
		return fmt.Errorf("channel: get connection: %w", err)
	}

	adapter, ok := s.adapters[conn.Provider]
	if !ok {
		return fmt.Errorf("channel: adapter not found for provider %s", conn.Provider)
	}

	if !slices.Contains(adapter.Capabilities(), capability) {
		return domain.ErrNotImplemented
	}

	done, err := s.breaker.Allow(conn.OrgID + ":" + conn.Provider)
	if err != nil {
		return fmt.Errorf("channel: circuit breaker: %w", err)
	}

	job := domain.SyncJob{
		ID:           uuid.NewString(),
		OrgID:        conn.OrgID,
		ConnectionID: connectionID,
		JobType:      jobType,
		Status:       domain.StatusRunning,
		Payload:      payload,
		ScheduledAt:  time.Now(),
	}
	if err := s.jobs.Create(ctx, job); err != nil {
		return fmt.Errorf("channel: create sync job: %w", err)
	}

	err = fn(adapter)
	success := err == nil
	done(success)

	if !success {
		s.log.Error("sync job failed", "job_id", job.ID, "err", err)
		_ = s.jobs.UpdateStatus(ctx, job.ID, domain.StatusFailed, nil, err.Error())
		_ = s.publisher.PublishSyncFailed(ctx, job)
		return err
	}

	_ = s.jobs.UpdateStatus(ctx, job.ID, domain.StatusSucceeded, nil, "")
	_ = s.connRepo.UpdateLastSync(ctx, connectionID, time.Now())
	_ = s.publisher.PublishSyncSucceeded(ctx, job)

	return nil
}
