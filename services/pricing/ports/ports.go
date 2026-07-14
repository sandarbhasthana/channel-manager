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

// PromoRepository persists promotional discount codes.
//
// Redeem is deliberately not expressible as Get-then-Save: incrementing the
// usage counter must be atomic against concurrent redemptions of the same code.
type PromoRepository interface {
	Create(ctx context.Context, p domain.PromoCode) (domain.PromoCode, error)
	GetByCode(ctx context.Context, code string) (domain.PromoCode, error)
	GetByID(ctx context.Context, id string) (domain.PromoCode, error)
	ListByOrg(ctx context.Context) ([]domain.PromoCode, error)
	Update(ctx context.Context, p domain.PromoCode) (domain.PromoCode, error)
	Delete(ctx context.Context, id string) error

	// Redeem atomically increments the usage counter for code, but only while
	// the code is active, within its validity window, in scope for propertyID,
	// and has redemptions remaining. It returns the updated code, or
	// domain.ErrPromoExhausted (and friends) when those conditions do not hold.
	//
	// This is the sole writer of `uses`. Callers must not read, decide, then
	// write — that is the race max_uses cannot survive.
	Redeem(ctx context.Context, code, propertyID string, at time.Time) (domain.PromoCode, error)

	// ReleaseRedemption decrements the counter, compensating when a booking
	// that consumed a redemption subsequently fails. Never drops below zero.
	ReleaseRedemption(ctx context.Context, code string) error
}
