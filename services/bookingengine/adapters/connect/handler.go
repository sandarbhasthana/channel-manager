// Package connect implements the Connect-RPC handler for the booking engine.
package connect

import (
	"context"
	"errors"
	"strconv"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	commonv1 "github.com/channel-manager/channel-manager/gen/go/common/v1"
	bev1 "github.com/channel-manager/channel-manager/gen/go/bookingengine/v1"
	"github.com/channel-manager/channel-manager/gen/go/bookingengine/v1/bookingenginev1connect"
	"github.com/channel-manager/channel-manager/services/bookingengine/domain"
	"github.com/channel-manager/channel-manager/services/bookingengine/usecases"
)

// Handler serves the BookingEngineService.
type Handler struct {
	bookingenginev1connect.UnimplementedBookingEngineServiceHandler
	svc *usecases.Service
}

// NewHandler creates a booking-engine Connect handler.
func NewHandler(svc *usecases.Service) *Handler {
	return &Handler{svc: svc}
}

var _ bookingenginev1connect.BookingEngineServiceHandler = (*Handler)(nil)

func (h *Handler) ListDirectReservations(
	ctx context.Context, req *connect.Request[bev1.ListDirectReservationsRequest],
) (*connect.Response[bev1.ListDirectReservationsResponse], error) {
	msg := req.Msg
	if msg.GetPropertyId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("property_id is required"))
	}
	offset := parseToken(msg.GetPage().GetPageToken())
	items, next, err := h.svc.ListDirectReservations(ctx, msg.GetPropertyId(), int(msg.GetPage().GetPageSize()), offset)
	if err != nil {
		return nil, toConnectError(err)
	}
	resp := &bev1.ListDirectReservationsResponse{
		Reservations: make([]*bev1.DirectReservation, 0, len(items)),
		Page:         &commonv1.PageResponse{},
	}
	for _, d := range items {
		resp.Reservations = append(resp.Reservations, reservationToProto(d))
	}
	if next > 0 {
		resp.Page.NextPageToken = strconv.Itoa(next)
	}
	return connect.NewResponse(resp), nil
}

func (h *Handler) GetSettings(
	ctx context.Context, req *connect.Request[bev1.GetSettingsRequest],
) (*connect.Response[bev1.GetSettingsResponse], error) {
	if req.Msg.GetPropertyId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("property_id is required"))
	}
	s, err := h.svc.GetSettings(ctx, req.Msg.GetPropertyId())
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&bev1.GetSettingsResponse{Settings: settingsToProto(s)}), nil
}

func (h *Handler) UpdateSettings(
	ctx context.Context, req *connect.Request[bev1.UpdateSettingsRequest],
) (*connect.Response[bev1.UpdateSettingsResponse], error) {
	if req.Msg.GetPropertyId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("property_id is required"))
	}
	s, err := h.svc.UpdateSettings(ctx, domain.Settings{
		PropertyID:           req.Msg.GetPropertyId(),
		DirectChannelEnabled: req.Msg.GetDirectChannelEnabled(),
		Route:                req.Msg.GetBookingRoute(),
		Percent:              int(req.Msg.GetBookingRoutePercent()),
	})
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&bev1.UpdateSettingsResponse{Settings: settingsToProto(s)}), nil
}

// parseToken reads an offset cursor. A malformed token starts from the top
// rather than erroring — the worst case is re-showing the first page.
func parseToken(tok string) int {
	if tok == "" {
		return 0
	}
	n, err := strconv.Atoi(tok)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

func toConnectError(err error) error {
	switch {
	case errors.Is(err, domain.ErrPropertyNotFound):
		return connect.NewError(connect.CodeNotFound, err)
	case errors.Is(err, domain.ErrInvalidSettings):
		return connect.NewError(connect.CodeInvalidArgument, err)
	default:
		return connect.NewError(connect.CodeInternal, err)
	}
}

func reservationToProto(d domain.DirectReservation) *bev1.DirectReservation {
	return &bev1.DirectReservation{
		Id:               d.ID,
		PropertyId:       d.PropertyID,
		ConfirmationCode: d.ConfirmationCode,
		GuestName:        d.GuestName,
		CheckIn:          dateToProto(d.CheckIn),
		CheckOut:         dateToProto(d.CheckOut),
		Status:           d.Status,
		Total:            &commonv1.Money{AmountMinor: d.TotalMinor, Currency: d.Currency},
		BookedAt:         timestamppb.New(d.BookedAt),
	}
}

func settingsToProto(s domain.Settings) *bev1.BookingEngineSettings {
	return &bev1.BookingEngineSettings{
		PropertyId:           s.PropertyID,
		DirectChannelEnabled: s.DirectChannelEnabled,
		BookingRoute:         s.Route,
		BookingRoutePercent:  int32(s.Percent),
	}
}

func dateToProto(t time.Time) *commonv1.CalendarDate {
	if t.IsZero() {
		return nil
	}
	return &commonv1.CalendarDate{
		Year:  int32(t.Year()),
		Month: int32(t.Month()),
		Day:   int32(t.Day()),
	}
}
