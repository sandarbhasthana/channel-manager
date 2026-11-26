package main

import (
	"context"

	bedomain "github.com/channel-manager/channel-manager/services/bookingengine/domain"
	bookingengineusecases "github.com/channel-manager/channel-manager/services/bookingengine/usecases"
	integrationusecases "github.com/channel-manager/channel-manager/services/integration/usecases"
	pricingpostgres "github.com/channel-manager/channel-manager/services/pricing/adapters/postgres"
)

// Adapters from the stores the standalone dashboard uses to the small
// interfaces the integration service takes (integrationusecases.DashboardStores),
// so PMS users reach the same data through the PMS endpoint.

type rateRuleAdapter struct {
	repo *pricingpostgres.ChannelRateRepository
}

func (a rateRuleAdapter) List(ctx context.Context, propertyID string) ([]integrationusecases.RateRule, error) {
	rules, err := a.repo.List(ctx, propertyID)
	if err != nil {
		return nil, err
	}
	out := make([]integrationusecases.RateRule, 0, len(rules))
	for _, r := range rules {
		out = append(out, integrationusecases.RateRule{RoomTypeID: r.RoomTypeID, ChannelID: r.ChannelID, AdjustPct: r.AdjustPct})
	}
	return out, nil
}

func (a rateRuleAdapter) Upsert(ctx context.Context, propertyID string, rules []integrationusecases.RateRule) error {
	in := make([]pricingpostgres.ChannelRateRule, 0, len(rules))
	for _, r := range rules {
		in = append(in, pricingpostgres.ChannelRateRule{RoomTypeID: r.RoomTypeID, ChannelID: r.ChannelID, AdjustPct: r.AdjustPct})
	}
	return a.repo.Upsert(ctx, propertyID, in)
}

type bookingEngineAdapter struct {
	svc *bookingengineusecases.Service
}

func (a bookingEngineAdapter) GetSettings(ctx context.Context, propertyID string) (integrationusecases.BookingEngineSettings, error) {
	s, err := a.svc.GetSettings(ctx, propertyID)
	if err != nil {
		return integrationusecases.BookingEngineSettings{}, err
	}
	return integrationusecases.BookingEngineSettings{DirectChannelEnabled: s.DirectChannelEnabled, BookingRoute: s.Route, BookingRoutePercent: s.Percent}, nil
}

func (a bookingEngineAdapter) UpdateSettings(ctx context.Context, propertyID string, in integrationusecases.BookingEngineSettings) (integrationusecases.BookingEngineSettings, error) {
	s, err := a.svc.UpdateSettings(ctx, bedomain.Settings{
		PropertyID:           propertyID,
		DirectChannelEnabled: in.DirectChannelEnabled,
		Route:                in.BookingRoute,
		Percent:              in.BookingRoutePercent,
	})
	if err != nil {
		return integrationusecases.BookingEngineSettings{}, err
	}
	return integrationusecases.BookingEngineSettings{DirectChannelEnabled: s.DirectChannelEnabled, BookingRoute: s.Route, BookingRoutePercent: s.Percent}, nil
}
