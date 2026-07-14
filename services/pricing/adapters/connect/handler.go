package connect

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	pricingv1 "github.com/channel-manager/channel-manager/gen/go/pricing/v1"
	"github.com/channel-manager/channel-manager/gen/go/pricing/v1/pricingv1connect"
	"github.com/channel-manager/channel-manager/services/pricing/domain"
	"github.com/channel-manager/channel-manager/services/pricing/usecases"
)

type Handler struct {
	pricingv1connect.UnimplementedPricingServiceHandler
	svc    *usecases.PricingService
	promos *usecases.PromoService
}

func NewHandler(svc *usecases.PricingService, promos *usecases.PromoService) *Handler {
	return &Handler{svc: svc, promos: promos}
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

// ── Promo codes (coupons) ───────────────────────────────────────────────────

func (h *Handler) ListPromoCodes(ctx context.Context, req *connect.Request[pricingv1.ListPromoCodesRequest]) (*connect.Response[pricingv1.ListPromoCodesResponse], error) {
	promos, err := h.promos.ListPromos(ctx)
	if err != nil {
		return nil, promoConnectError(err)
	}
	out := make([]*pricingv1.PromoCode, 0, len(promos))
	for _, p := range promos {
		out = append(out, promoToProto(p))
	}
	return connect.NewResponse(&pricingv1.ListPromoCodesResponse{PromoCodes: out}), nil
}

func (h *Handler) CreatePromoCode(ctx context.Context, req *connect.Request[pricingv1.CreatePromoCodeRequest]) (*connect.Response[pricingv1.CreatePromoCodeResponse], error) {
	created, err := h.promos.CreatePromo(ctx, promoFromProto(req.Msg.GetPromoCode()))
	if err != nil {
		return nil, promoConnectError(err)
	}
	return connect.NewResponse(&pricingv1.CreatePromoCodeResponse{PromoCode: promoToProto(created)}), nil
}

func (h *Handler) UpdatePromoCode(ctx context.Context, req *connect.Request[pricingv1.UpdatePromoCodeRequest]) (*connect.Response[pricingv1.UpdatePromoCodeResponse], error) {
	updated, err := h.promos.UpdatePromo(ctx, promoFromProto(req.Msg.GetPromoCode()))
	if err != nil {
		return nil, promoConnectError(err)
	}
	return connect.NewResponse(&pricingv1.UpdatePromoCodeResponse{PromoCode: promoToProto(updated)}), nil
}

func (h *Handler) DeletePromoCode(ctx context.Context, req *connect.Request[pricingv1.DeletePromoCodeRequest]) (*connect.Response[pricingv1.DeletePromoCodeResponse], error) {
	if err := h.promos.DeletePromo(ctx, req.Msg.GetId()); err != nil {
		return nil, promoConnectError(err)
	}
	return connect.NewResponse(&pricingv1.DeletePromoCodeResponse{}), nil
}

func promoConnectError(err error) error {
	switch {
	case errors.Is(err, domain.ErrPromoNotFound):
		return connect.NewError(connect.CodeNotFound, err)
	default:
		return connect.NewError(connect.CodeInvalidArgument, err)
	}
}

func promoToProto(p domain.PromoCode) *pricingv1.PromoCode {
	pc := &pricingv1.PromoCode{
		Id:          p.ID,
		PropertyId:  p.PropertyID,
		Code:        p.Code,
		Description: p.Description,
		DiscountPct: p.DiscountPct,
		Uses:        int32(p.Uses),
		IsActive:    p.IsActive,
		CreatedAt:   timestamppb.New(p.CreatedAt),
		UpdatedAt:   timestamppb.New(p.UpdatedAt),
	}
	if p.MaxUses != nil {
		pc.MaxUses = int32(*p.MaxUses)
	}
	if p.ValidFrom != nil {
		pc.ValidFrom = timestamppb.New(*p.ValidFrom)
	}
	if p.ValidUntil != nil {
		pc.ValidUntil = timestamppb.New(*p.ValidUntil)
	}
	return pc
}

func promoFromProto(pc *pricingv1.PromoCode) domain.PromoCode {
	p := domain.PromoCode{
		ID:          pc.GetId(),
		PropertyID:  pc.GetPropertyId(),
		Code:        pc.GetCode(),
		Description: pc.GetDescription(),
		DiscountPct: pc.GetDiscountPct(),
		IsActive:    pc.GetIsActive(),
	}
	// max_uses 0 on the wire means unlimited (nil); >0 sets a limit.
	if m := pc.GetMaxUses(); m > 0 {
		v := int(m)
		p.MaxUses = &v
	}
	if t := pc.GetValidFrom(); t != nil {
		vt := t.AsTime()
		p.ValidFrom = &vt
	}
	if t := pc.GetValidUntil(); t != nil {
		vt := t.AsTime()
		p.ValidUntil = &vt
	}
	return p
}

