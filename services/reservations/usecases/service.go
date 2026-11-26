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
// PMS (e.g. an OTA booking the integration service fetched) and tells the PMS:
// reservation.created for a booking it has not seen, reservation.updated when
// a known booking's stay, status or price changed, nothing when it is the
// same. The bool is true only for a new booking. The OTA pull re-reads the
// last 30 days on every sync, so announcing every row as created made the
// PMS import each booking again (BUG-064).
//
// For a booking the PMS ALREADY has (a direct booking made through the
// storefront), use RecordReservation instead — publishing that back makes the
// PMS duplicate it.
func (s *ReservationService) IngestReservation(ctx context.Context, res *domain.Reservation, idempotencyKey string) (string, bool, error) {
	return s.persist(ctx, res, idempotencyKey, true)
}

// RecordReservation persists a canonical reservation for a booking that already
// exists in the PMS, WITHOUT publishing anything. The PMS is the origin of the
// stay, so propagating it back would make the PMS create a duplicate
// reservation.
func (s *ReservationService) RecordReservation(ctx context.Context, res *domain.Reservation, idempotencyKey string) (string, bool, error) {
	return s.persist(ctx, res, idempotencyKey, false)
}

// persist saves a reservation and, only when publish is true, tells the PMS
// what changed.
func (s *ReservationService) persist(ctx context.Context, res *domain.Reservation, idempotencyKey string, publish bool) (string, bool, error) {
	if idempotencyKey != "" {
		if _, ok := s.seenKeys[idempotencyKey]; ok {
			return "", false, ErrDuplicateRequest
		}
	}
	if res.Status == "" {
		res.Status = "confirmed"
	}
	res.AwaitPMS = publish
	result, err := s.repo.Save(ctx, res)
	if err != nil {
		return "", false, fmt.Errorf("reservations: ingest: %w", err)
	}
	if idempotencyKey != "" {
		s.seenKeys[idempotencyKey] = struct{}{}
	}
	if publish && result != domain.SaveUnchanged {
		s.deliver(ctx, res, result == domain.SaveInserted)
	}
	_ = uuid.NewString() // event id reserved for outbox
	return res.ID, result == domain.SaveInserted, nil
}

// deliver announces a booking to the PMS and records whether it took it. A
// booking the PMS didn't take stays pending and is resent by RedeliverPending.
func (s *ReservationService) deliver(ctx context.Context, res *domain.Reservation, created bool) bool {
	var err error
	if created {
		err = s.publisher.PublishReservationCreated(ctx, res)
	} else {
		err = s.publisher.PublishReservationUpdated(ctx, res)
	}
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	if markErr := s.repo.MarkDelivery(ctx, res.ID, err == nil, msg); markErr != nil {
		// The booking stays pending (its last known state) and is resent,
		// which the PMS treats as a no-op once it has it.
		return false
	}
	return err == nil
}

// RedeliverPending resends a property's bookings the PMS hasn't taken yet
// (it was unreachable, or failed, when they were first announced). Each is
// announced as new; the PMS imports an OTA booking once, by its id, so a
// booking it did take in the meantime is not duplicated.
func (s *ReservationService) RedeliverPending(ctx context.Context, propertyID string) (delivered, stillPending int, err error) {
	pending, err := s.repo.ListUndelivered(ctx, propertyID)
	if err != nil {
		return 0, 0, err
	}
	for i := range pending {
		if s.deliver(ctx, &pending[i], true) {
			delivered++
		} else {
			stillPending++
		}
	}
	return delivered, stillPending, nil
}

// CancelReservation marks a reservation cancelled.
func (s *ReservationService) CancelReservation(ctx context.Context, id string) (*domain.Reservation, error) {
	if err := s.repo.UpdateStatus(ctx, id, "cancelled"); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}
