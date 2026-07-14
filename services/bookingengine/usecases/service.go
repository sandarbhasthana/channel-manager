// Package usecases orchestrates the booking-engine read model and settings.
package usecases

import (
	"context"
	"fmt"

	"github.com/channel-manager/channel-manager/services/bookingengine/domain"
	"github.com/channel-manager/channel-manager/services/bookingengine/ports"
)

// Pagination bounds for the direct-reservations list.
const (
	DefaultPageSize = 50
	MaxPageSize     = 200
)

// Service exposes booking-engine operations to the transport layer.
type Service struct {
	repo ports.Repository
}

// NewService creates a booking-engine service.
func NewService(repo ports.Repository) *Service {
	return &Service{repo: repo}
}

// ListDirectReservations returns one page of direct bookings and the offset of
// the next page (0 when the page was not full, i.e. no more rows).
func (s *Service) ListDirectReservations(ctx context.Context, propertyID string, pageSize, offset int) ([]domain.DirectReservation, int, error) {
	if pageSize <= 0 || pageSize > MaxPageSize {
		pageSize = DefaultPageSize
	}
	if offset < 0 {
		offset = 0
	}
	items, err := s.repo.ListDirectReservations(ctx, propertyID, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	next := 0
	if len(items) == pageSize {
		next = offset + pageSize
	}
	return items, next, nil
}

// GetSettings returns the property's booking-engine settings.
func (s *Service) GetSettings(ctx context.Context, propertyID string) (domain.Settings, error) {
	return s.repo.GetSettings(ctx, propertyID)
}

// UpdateSettings writes the booking-engine settings for a property after
// validating the route and percentage.
func (s *Service) UpdateSettings(ctx context.Context, in domain.Settings) (domain.Settings, error) {
	if in.Route != "pms" && in.Route != "cm" {
		return domain.Settings{}, fmt.Errorf("%w: booking_route must be 'pms' or 'cm'", domain.ErrInvalidSettings)
	}
	if in.Percent < 0 || in.Percent > 100 {
		return domain.Settings{}, fmt.Errorf("%w: booking_route_percent must be 0-100", domain.ErrInvalidSettings)
	}
	return s.repo.UpdateSettings(ctx, in)
}
