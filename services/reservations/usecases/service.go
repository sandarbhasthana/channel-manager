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

// IngestReservation persists a canonical reservation (from PMS or OTA fetch).
func (s *ReservationService) IngestReservation(ctx context.Context, res *domain.Reservation, idempotencyKey string) (string, bool, error) {
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
	_ = s.publisher.PublishReservationCreated(ctx, res)
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
