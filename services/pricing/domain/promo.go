package domain

import (
	"errors"
	"time"
)

// Promo rejection reasons. These are returned to the booking engine so it can
// tell a guest *why* a code did not apply, rather than silently charging full
// price.
var (
	ErrPromoNotFound    = errors.New("pricing: promo code not found")
	ErrPromoInactive    = errors.New("pricing: promo code is inactive")
	ErrPromoNotYetValid = errors.New("pricing: promo code is not yet valid")
	ErrPromoExpired     = errors.New("pricing: promo code has expired")
	ErrPromoExhausted   = errors.New("pricing: promo code usage limit reached")
	ErrPromoWrongScope  = errors.New("pricing: promo code does not apply to this property")
)

// PromoCode is a discount code owned by an organization.
//
// Channel Manager is the source of truth for both the definition and the
// redemption counter. A booking engine may evaluate Validate locally to decide
// what to show a guest, but must call redemption through Channel Manager: only
// a single writer can make MaxUses hold.
type PromoCode struct {
	ID    string `json:"id"`
	OrgID string `json:"org_id"`

	// PropertyID empty means the code applies to every property in the org.
	PropertyID string `json:"property_id,omitempty"`

	Code        string  `json:"code"`
	Description string  `json:"description,omitempty"`
	DiscountPct float64 `json:"discount_pct"`

	// MaxUses nil means unlimited.
	MaxUses *int `json:"max_uses,omitempty"`
	Uses    int  `json:"uses"`

	ValidFrom  *time.Time `json:"valid_from,omitempty"`
	ValidUntil *time.Time `json:"valid_until,omitempty"`
	IsActive   bool       `json:"is_active"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// AppliesToProperty reports whether p may be used for a booking at propertyID.
// An org-wide code (empty PropertyID) applies everywhere.
func (p PromoCode) AppliesToProperty(propertyID string) bool {
	return p.PropertyID == "" || p.PropertyID == propertyID
}

// Exhausted reports whether the code has no redemptions left.
func (p PromoCode) Exhausted() bool {
	return p.MaxUses != nil && p.Uses >= *p.MaxUses
}

// Validate evaluates every stateless rule plus the *observed* usage count,
// returning the specific reason a code does not apply.
//
// The Exhausted check here is advisory: `uses` may have moved between this read
// and a redemption. Redemption is therefore guarded by a conditional UPDATE in
// the repository, which is the only place `max_uses` is truly enforced. This
// method exists so a guest sees "code exhausted" while browsing rather than at
// the moment of payment.
func (p PromoCode) Validate(at time.Time, propertyID string) error {
	if !p.IsActive {
		return ErrPromoInactive
	}
	if !p.AppliesToProperty(propertyID) {
		return ErrPromoWrongScope
	}
	if p.ValidFrom != nil && at.Before(*p.ValidFrom) {
		return ErrPromoNotYetValid
	}
	if p.ValidUntil != nil && !at.Before(*p.ValidUntil) {
		return ErrPromoExpired
	}
	if p.Exhausted() {
		return ErrPromoExhausted
	}
	return nil
}

// DiscountedAmount applies the code's percentage to amount, rounded to cents so
// the figure cannot drift from what a payment processor is asked to charge.
func (p PromoCode) DiscountedAmount(amount float64) float64 {
	discounted := amount * (1 - p.DiscountPct/100)
	return float64(int64(discounted*100+0.5)) / 100
}

// LookupResult is the answer to "can this guest use this code right now",
// without consuming a redemption.
//
// Valid is a snapshot: another guest may take the last redemption a moment
// later. Redeem remains the authority.
type LookupResult struct {
	Promo  PromoCode `json:"promo"`
	Valid  bool      `json:"valid"`
	Reason error     `json:"-"`
}
