package usecases

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/channel-manager/channel-manager/services/pricing/domain"
	"github.com/channel-manager/channel-manager/services/pricing/ports"
)

// PromoService owns promotional codes: their definitions and, crucially, their
// redemption counters.
//
// A booking engine may read a code and evaluate domain.PromoCode.Validate for
// itself — that is how a guest is told "expired" while still browsing. But the
// counter is only ever moved here, through the repository's conditional UPDATE.
// Two writers make max_uses meaningless.
type PromoService struct {
	repo ports.PromoRepository
	now  func() time.Time
}

// NewPromoService creates a PromoService. A nil clock uses time.Now.
func NewPromoService(repo ports.PromoRepository, now func() time.Time) *PromoService {
	if now == nil {
		now = time.Now
	}
	return &PromoService{repo: repo, now: now}
}

// normalizeCode makes lookup insensitive to how a guest typed the code.
// Stored codes are upper-cased on create, so comparison is exact thereafter.
func normalizeCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

// CreatePromo registers a new code for the caller's organization.
func (s *PromoService) CreatePromo(ctx context.Context, p domain.PromoCode) (domain.PromoCode, error) {
	p.Code = normalizeCode(p.Code)
	if p.Code == "" {
		return domain.PromoCode{}, fmt.Errorf("pricing: code is required")
	}
	if p.DiscountPct <= 0 || p.DiscountPct > 100 {
		return domain.PromoCode{}, fmt.Errorf("pricing: discount_pct must be in (0, 100]")
	}
	if p.MaxUses != nil && *p.MaxUses <= 0 {
		return domain.PromoCode{}, fmt.Errorf("pricing: max_uses must be positive when set")
	}
	if p.ValidFrom != nil && p.ValidUntil != nil && !p.ValidUntil.After(*p.ValidFrom) {
		return domain.PromoCode{}, fmt.Errorf("pricing: valid_until must be after valid_from")
	}
	return s.repo.Create(ctx, p)
}

// ListPromos returns every code in the caller's organization.
func (s *PromoService) ListPromos(ctx context.Context) ([]domain.PromoCode, error) {
	return s.repo.ListByOrg(ctx)
}

// GetPromo returns one code by id.
func (s *PromoService) GetPromo(ctx context.Context, id string) (domain.PromoCode, error) {
	return s.repo.GetByID(ctx, id)
}

// UpdatePromo rewrites a code's mutable fields. `uses` is not among them.
func (s *PromoService) UpdatePromo(ctx context.Context, p domain.PromoCode) (domain.PromoCode, error) {
	if p.DiscountPct <= 0 || p.DiscountPct > 100 {
		return domain.PromoCode{}, fmt.Errorf("pricing: discount_pct must be in (0, 100]")
	}
	return s.repo.Update(ctx, p)
}

// DeletePromo removes a code.
func (s *PromoService) DeletePromo(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

// Lookup resolves a code and evaluates its rules for a property, without
// consuming a redemption.
//
// The Exhausted verdict here is a snapshot: another guest may take the last
// redemption a moment later. Callers must still treat Redeem as the authority.
func (s *PromoService) Lookup(ctx context.Context, code, propertyID string) (domain.LookupResult, error) {
	promo, err := s.repo.GetByCode(ctx, normalizeCode(code))
	if err != nil {
		return domain.LookupResult{}, err
	}
	if reason := promo.Validate(s.now(), propertyID); reason != nil {
		return domain.LookupResult{Promo: promo, Valid: false, Reason: reason}, nil
	}
	return domain.LookupResult{Promo: promo, Valid: true}, nil
}

// Redeem consumes one use of a code, atomically.
//
// Returns domain.ErrPromoExhausted, ErrPromoExpired, ErrPromoWrongScope,
// ErrPromoInactive, ErrPromoNotYetValid, or ErrPromoNotFound when the code
// cannot be redeemed. A successful call has already incremented the counter.
func (s *PromoService) Redeem(ctx context.Context, code, propertyID string) (domain.PromoCode, error) {
	return s.repo.Redeem(ctx, normalizeCode(code), propertyID, s.now())
}

// ReleaseRedemption returns a consumed redemption, compensating a booking that
// failed after the code was redeemed.
func (s *PromoService) ReleaseRedemption(ctx context.Context, code string) error {
	return s.repo.ReleaseRedemption(ctx, normalizeCode(code))
}
