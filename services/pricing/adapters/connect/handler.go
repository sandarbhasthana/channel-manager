package connect

import (
	"context"

	"connectrpc.com/connect"
	pricingv1 "github.com/channel-manager/channel-manager/gen/go/pricing/v1"
	"github.com/channel-manager/channel-manager/gen/go/pricing/v1/pricingv1connect"
)

type Handler struct {
	pricingv1connect.UnimplementedPricingServiceHandler
}

func NewHandler() *Handler {
	return &Handler{}
}

func (h *Handler) GetRates(ctx context.Context, req *connect.Request[pricingv1.GetRatesRequest]) (*connect.Response[pricingv1.GetRatesResponse], error) {
	return connect.NewResponse(&pricingv1.GetRatesResponse{
		Points: []*pricingv1.RatePoint{},
	}), nil
}

func (h *Handler) ListRatePlans(ctx context.Context, req *connect.Request[pricingv1.ListRatePlansRequest]) (*connect.Response[pricingv1.ListRatePlansResponse], error) {
	return connect.NewResponse(&pricingv1.ListRatePlansResponse{
		Plans: []*pricingv1.RatePlan{},
	}), nil
}

func (h *Handler) BulkUpsertRates(ctx context.Context, req *connect.Request[pricingv1.BulkUpsertRatesRequest]) (*connect.Response[pricingv1.BulkUpsertRatesResponse], error) {
	return connect.NewResponse(&pricingv1.BulkUpsertRatesResponse{
		RowsAffected: int32(len(req.Msg.Points)),
		EventId:      "mock-event-id",
	}), nil
}

