package usecases

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/channel-manager/channel-manager/services/reservations/domain"
	"github.com/channel-manager/channel-manager/services/reservations/ports"
)

// ErrDuplicateRequest is returned when an idempotency key was already processed.
var ErrDuplicateRequest = errors.New("reservations: duplicate request")

// ReservationService orchestrates reservation operations.
type ReservationService struct {
	repo      ports.ReservationRepository
	publisher ports.ReservationEventPublisher
	seenKeys  map[string]struct{} // dev idempotency; replace with Redis in production
}

// NewReservationService creates a ReservationService.
func NewReservationService(repo ports.ReservationRepository, publisher ports.ReservationEventPublisher) *ReservationService {
	return &ReservationService{
		repo:      repo,
		publisher: publisher,
		seenKeys:  make(map[string]struct{}),
	}
}

// GetReservation returns a reservation by ID.
func (s *ReservationService) GetReservation(ctx context.Context, id string) (*domain.Reservation, error) {
	return s.repo.GetByID(ctx, id)
}

// ListReservations lists reservations for a property.
func (s *ReservationService) ListReservations(ctx context.Context, propertyID string) ([]domain.Reservation, error) {
	return s.repo.ListByProperty(ctx, propertyID)
}

// IngestReservation persists a canonical reservation that ORIGINATED outside the
// PMS (e.g. an OTA booking the integration service fetched) and publishes
// reservation.created so the PMS is notified and creates it. For a booking the
// PMS ALREADY has (a direct booking made through the storefront), use
// RecordReservation instead — publishing that back makes the PMS duplicate it.
func (s *ReservationService) IngestReservation(ctx context.Context, res *domain.Reservation, idempotencyKey string) (string, bool, error) {
	return s.persist(ctx, res, idempotencyKey, true)
}

// RecordReservation persists a canonical reservation for a booking that already
// exists in the PMS, WITHOUT publishing reservation.created. The PMS is the
// origin of the stay, so propagating it back would make the PMS create a
// duplicate reservation.
func (s *ReservationService) RecordReservation(ctx context.Context, res *domain.Reservation, idempotencyKey string) (string, bool, error) {
	return s.persist(ctx, res, idempotencyKey, false)
}

// persist saves a reservation and, only when publish is true, emits
// reservation.created to the PMS webhook.
func (s *ReservationService) persist(ctx context.Context, res *domain.Reservation, idempotencyKey string, publish bool) (string, bool, error) {
	if idempotencyKey != "" {
		if _, ok := s.seenKeys[idempotencyKey]; ok {
			return "", false, ErrDuplicateRequest
		}
	}
	if res.Status == "" {
		res.Status = "confirmed"
	}
	if err := s.repo.Save(ctx, res); err != nil {
		return "", false, fmt.Errorf("reservations: ingest: %w", err)
	}
	if idempotencyKey != "" {
		s.seenKeys[idempotencyKey] = struct{}{}
	}
	if publish {
		_ = s.publisher.PublishReservationCreated(ctx, res)
	}
	_ = uuid.NewString() // event id reserved for outbox
	return res.ID, true, nil
}

// CancelReservation marks a reservation cancelled.
func (s *ReservationService) CancelReservation(ctx context.Context, id string) (*domain.Reservation, error) {
	if err := s.repo.UpdateStatus(ctx, id, "cancelled"); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}
