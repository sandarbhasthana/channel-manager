package ports

import (
	"context"
	"time"

	"github.com/channel-manager/channel-manager/services/pricing/domain"
)

// RateRepository provides persistence for rate data.
type RateRepository interface {
	Get(ctx context.Context, propertyID, roomTypeID, ratePlanID string, date time.Time) (*domain.RateDay, error)
	ListByRange(ctx context.Context, propertyID, roomTypeID, ratePlanID string, from, to time.Time) ([]domain.RateDay, error)
	Save(ctx context.Context, day *domain.RateDay) error
	SaveBatch(ctx context.Context, days []domain.RateDay) error
}

// RateEventPublisher publishes rate change events.
type RateEventPublisher interface {
	PublishRateUpdated(ctx context.Context, day *domain.RateDay) error
}
