package usecases

import (
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
