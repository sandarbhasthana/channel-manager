package usecases

import (
	"context"

	"github.com/channel-manager/channel-manager/services/pricing/domain"
	"github.com/channel-manager/channel-manager/services/pricing/ports"
)

// PricingService orchestrates rate and pricing operations.
type PricingService struct {
	repo      ports.RateRepository
	publisher ports.RateEventPublisher
}

// NewPricingService creates a new PricingService.
func NewPricingService(repo ports.RateRepository, publisher ports.RateEventPublisher) *PricingService {
	return &PricingService{
		repo:      repo,
		publisher: publisher,
	}
}

// BulkUpsertRates saves a batch of rate days.
func (s *PricingService) BulkUpsertRates(ctx context.Context, days []domain.RateDay) error {
	return s.repo.SaveBatch(ctx, days)
}

