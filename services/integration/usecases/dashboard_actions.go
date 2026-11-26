package usecases

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	chdomain "github.com/channel-manager/channel-manager/services/channel/domain"
	pmsdomain "github.com/channel-manager/channel-manager/services/pms/domain"
	pricingdomain "github.com/channel-manager/channel-manager/services/pricing/domain"
)

// Dashboard features on the PMS endpoint.
//
// Bundled tenants use the Channel Manager only through the PMS: they have no
// WorkOS login, so the Connect-RPC services and /admin/* routes the standalone
// dashboard uses are out of reach for them. These actions expose the same
// features — per-channel rate rules, booking-engine settings, promo codes and
// the default property — on POST /api/integrations/pms/{propertyId}, backed by
// the very same stores, so both front ends read and write one set of data.
//
// The stores are injected through small interfaces (wired in apps/api) so this
// module does not take on the pricing / booking-engine adapters as
// dependencies.

// RateRule is one per-channel adjustment: the channel receives
// base * (1 + AdjustPct/100). ChannelID is the connection id; the booking
// engine's rule is stored under DirectChannelID.
type RateRule struct {
	RoomTypeID string  `json:"room_type_id"`
	ChannelID  string  `json:"channel_id"`
	AdjustPct  float64 `json:"adjust_pct"`
}

// DirectChannelID is the id the standalone dashboard stores the booking
// engine's rate rule under.
const DirectChannelID = "00000000-0000-0000-0000-000000000000"

// RateRuleStore is pricingpostgres.ChannelRateRepository, adapted.
type RateRuleStore interface {
	List(ctx context.Context, propertyID string) ([]RateRule, error)
	Upsert(ctx context.Context, propertyID string, rules []RateRule) error
}

// BookingEngineSettings mirrors bookingengine/domain.Settings.
type BookingEngineSettings struct {
	DirectChannelEnabled bool   `json:"direct_channel_enabled"`
	BookingRoute         string `json:"booking_route"`
	BookingRoutePercent  int    `json:"booking_route_percent"`
}

// BookingEngineStore is the booking-engine usecase service, adapted.
type BookingEngineStore interface {
	GetSettings(ctx context.Context, propertyID string) (BookingEngineSettings, error)
	UpdateSettings(ctx context.Context, propertyID string, s BookingEngineSettings) (BookingEngineSettings, error)
}

// PromoStore is the pricing PromoService.
type PromoStore interface {
	ListPromos(ctx context.Context) ([]pricingdomain.PromoCode, error)
	CreatePromo(ctx context.Context, p pricingdomain.PromoCode) (pricingdomain.PromoCode, error)
	UpdatePromo(ctx context.Context, p pricingdomain.PromoCode) (pricingdomain.PromoCode, error)
	DeletePromo(ctx context.Context, id string) error
}

// DefaultPropertyStore is the pms PropertyRepository.
type DefaultPropertyStore interface {
	ListListings(ctx context.Context) ([]pmsdomain.PropertyListing, error)
	SetDefault(ctx context.Context, id string) error
}

// DashboardStores groups the optional dependencies. Any nil store makes its
// actions answer "not configured" instead of panicking.
type DashboardStores struct {
	RateRules     RateRuleStore
	BookingEngine BookingEngineStore
	Promos        PromoStore
	Properties    DefaultPropertyStore
}

// WithDashboard attaches the stores. Kept off NewService so existing callers
// and tests are unchanged.
func (s *Service) WithDashboard(d DashboardStores) *Service {
	s.dash = d
	return s
}

var errNotConfigured = errors.New("this action is not configured on this deployment")

// dispatchDashboard handles the dashboard actions. ok=false means the action
// is not one of them.
func (s *Service) dispatchDashboard(ctx context.Context, propertyID, action string, body map[string]any) (any, bool, error) {
	switch action {
	case "get_channel_rate_rules":
		if s.dash.RateRules == nil {
			return nil, true, errNotConfigured
		}
		rules, err := s.dash.RateRules.List(ctx, propertyID)
		if err != nil {
			return nil, true, err
		}
		if rules == nil {
			rules = []RateRule{}
		}
		return map[string]any{"rules": rules}, true, nil

	case "save_channel_rate_rules":
		if s.dash.RateRules == nil {
			return nil, true, errNotConfigured
		}
		rules, err := parseRateRules(body["rules"])
		if err != nil {
			return nil, true, err
		}
		if err := s.dash.RateRules.Upsert(ctx, propertyID, rules); err != nil {
			return nil, true, err
		}
		return map[string]any{"saved": len(rules)}, true, nil

	case "get_booking_engine_settings":
		if s.dash.BookingEngine == nil {
			return nil, true, errNotConfigured
		}
		st, err := s.dash.BookingEngine.GetSettings(ctx, propertyID)
		if err != nil {
			return nil, true, err
		}
		return map[string]any{"settings": st}, true, nil

	case "update_booking_engine_settings":
		if s.dash.BookingEngine == nil {
			return nil, true, errNotConfigured
		}
		current, err := s.dash.BookingEngine.GetSettings(ctx, propertyID)
		if err != nil {
			return nil, true, err
		}
		// Partial update: only the fields sent change.
		if v, ok := body["direct_channel_enabled"].(bool); ok {
			current.DirectChannelEnabled = v
		}
		if v, ok := body["booking_route"].(string); ok {
			current.BookingRoute = v
		}
		if v, ok := body["booking_route_percent"].(float64); ok {
			current.BookingRoutePercent = int(v)
		}
		if current.BookingRoute == "" {
			current.BookingRoute = "pms"
		}
		st, err := s.dash.BookingEngine.UpdateSettings(ctx, propertyID, current)
		if err != nil {
			return nil, true, err
		}
		return map[string]any{"settings": st}, true, nil

	case "list_promo_codes":
		if s.dash.Promos == nil {
			return nil, true, errNotConfigured
		}
		all, err := s.dash.Promos.ListPromos(ctx)
		if err != nil {
			return nil, true, err
		}
		// This property's codes plus org-wide ones; another property's codes are
		// not this property's business.
		out := make([]pricingdomain.PromoCode, 0, len(all))
		for _, p := range all {
			if p.AppliesToProperty(propertyID) {
				out = append(out, p)
			}
		}
		return map[string]any{"promo_codes": out}, true, nil

	case "create_promo_code", "update_promo_code":
		if s.dash.Promos == nil {
			return nil, true, errNotConfigured
		}
		p, err := parsePromo(body, propertyID)
		if err != nil {
			return nil, true, err
		}
		if action == "update_promo_code" {
			if p.ID == "" {
				return nil, true, errors.New("id is required")
			}
			if err := s.requirePromoOfProperty(ctx, p.ID, propertyID); err != nil {
				return nil, true, err
			}
			saved, err := s.dash.Promos.UpdatePromo(ctx, p)
			if err != nil {
				return nil, true, err
			}
			return map[string]any{"promo_code": saved}, true, nil
		}
		p.ID = ""
		saved, err := s.dash.Promos.CreatePromo(ctx, p)
		if err != nil {
			return nil, true, err
		}
		return map[string]any{"promo_code": saved}, true, nil

	case "delete_promo_code":
		if s.dash.Promos == nil {
			return nil, true, errNotConfigured
		}
		id, _ := body["id"].(string)
		if id == "" {
			return nil, true, errors.New("id is required")
		}
		if err := s.requirePromoOfProperty(ctx, id, propertyID); err != nil {
			return nil, true, err
		}
		if err := s.dash.Promos.DeletePromo(ctx, id); err != nil {
			return nil, true, err
		}
		return map[string]any{"id": id, "deleted": true}, true, nil

	case "get_default_property":
		if s.dash.Properties == nil {
			return nil, true, errNotConfigured
		}
		listings, err := s.dash.Properties.ListListings(ctx)
		if err != nil {
			return nil, true, err
		}
		var def *string
		for _, l := range listings {
			if l.IsDefault {
				id := l.ExternalID
				if id == "" {
					id = l.ID
				}
				def = &id
			}
		}
		return map[string]any{"default_property_id": def, "is_default": func() bool {
			for _, l := range listings {
				if l.ID == propertyID {
					return l.IsDefault
				}
			}
			return false
		}()}, true, nil

	case "set_default_property":
		if s.dash.Properties == nil {
			return nil, true, errNotConfigured
		}
		// Only "this property" can be made the default from a property-scoped
		// endpoint; picking another property is that property's business.
		if err := s.dash.Properties.SetDefault(ctx, propertyID); err != nil {
			return nil, true, err
		}
		return map[string]any{"is_default": true}, true, nil
	}
	return nil, false, nil
}

// requirePromoOfProperty refuses to edit a code that belongs to a different
// property. Org-wide codes are editable from any property of the org.
func (s *Service) requirePromoOfProperty(ctx context.Context, id, propertyID string) error {
	all, err := s.dash.Promos.ListPromos(ctx)
	if err != nil {
		return err
	}
	for _, p := range all {
		if p.ID == id {
			if !p.AppliesToProperty(propertyID) {
				return errors.New("promo code not found")
			}
			return nil
		}
	}
	return errors.New("promo code not found")
}

func parseRateRules(v any) ([]RateRule, error) {
	raw, ok := v.([]any)
	if !ok {
		return nil, errors.New("rules must be an array")
	}
	out := make([]RateRule, 0, len(raw))
	for i, item := range raw {
		m, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("rules[%d] must be an object", i)
		}
		rt, _ := m["room_type_id"].(string)
		ch, _ := m["channel_id"].(string)
		pct, ok := m["adjust_pct"].(float64)
		if rt == "" || ch == "" || !ok {
			return nil, fmt.Errorf("rules[%d] needs room_type_id, channel_id and adjust_pct", i)
		}
		// -100 would give the room away; anything past +300 is a typo.
		if pct <= -100 || pct > 300 || math.IsNaN(pct) {
			return nil, fmt.Errorf("rules[%d]: adjust_pct must be above -100 and at most 300", i)
		}
		out = append(out, RateRule{RoomTypeID: rt, ChannelID: ch, AdjustPct: pct})
	}
	return out, nil
}

func parsePromo(body map[string]any, propertyID string) (pricingdomain.PromoCode, error) {
	p := pricingdomain.PromoCode{}
	p.ID, _ = body["id"].(string)
	p.Code, _ = body["code"].(string)
	p.Code = strings.TrimSpace(p.Code)
	if p.Code == "" {
		return p, errors.New("code is required")
	}
	p.Description, _ = body["description"].(string)
	pct, ok := body["discount_pct"].(float64)
	if !ok {
		return p, errors.New("discount_pct is required")
	}
	p.DiscountPct = pct
	if v, ok := body["max_uses"].(float64); ok && v > 0 {
		n := int(v)
		p.MaxUses = &n
	}
	for key, dst := range map[string]**time.Time{"valid_from": &p.ValidFrom, "valid_until": &p.ValidUntil} {
		if s, ok := body[key].(string); ok && s != "" {
			t, err := time.Parse(time.RFC3339, s)
			if err != nil {
				if t, err = time.Parse("2006-01-02", s); err != nil {
					return p, fmt.Errorf("%s must be a date", key)
				}
			}
			*dst = &t
		}
	}
	p.IsActive = true
	if v, ok := body["is_active"].(bool); ok {
		p.IsActive = v
	}
	// Org-wide only when asked for; otherwise the code belongs to this property.
	if orgWide, _ := body["org_wide"].(bool); !orgWide {
		p.PropertyID = propertyID
	}
	return p, nil
}

// applyRateRules returns the rate updates one channel should receive: each
// night's price adjusted by that channel's rule for the room type, rounded to
// cents. The input is not modified (it is reused for every channel).
func applyRateRules(updates []chdomain.RateUpdate, connectionID string, rules []RateRule) []chdomain.RateUpdate {
	pct := map[string]float64{}
	for _, r := range rules {
		if r.ChannelID == connectionID {
			pct[r.RoomTypeID] = r.AdjustPct
		}
	}
	if len(pct) == 0 {
		return updates
	}
	out := make([]chdomain.RateUpdate, len(updates))
	for i, u := range updates {
		if p, ok := pct[u.RoomTypeID]; ok && p != 0 {
			u.BaseRate = math.Round(u.BaseRate*(1+p/100)*100) / 100
		}
		out[i] = u
	}
	return out
}
