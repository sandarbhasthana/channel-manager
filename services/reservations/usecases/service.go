package usecases

import (
	"github.com/channel-manager/channel-manager/services/reservations/ports"
)

// ReservationService orchestrates reservation operations.
type ReservationService struct {
	repo      ports.ReservationRepository
	publisher ports.ReservationEventPublisher
}

// NewReservationService creates a new ReservationService.
func NewReservationService(repo ports.ReservationRepository, publisher ports.ReservationEventPublisher) *ReservationService {
	return &ReservationService{
		repo:      repo,
		publisher: publisher,
	}
}
