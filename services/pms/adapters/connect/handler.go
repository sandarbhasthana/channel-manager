// Package connect implements Connect-RPC handlers for the PMS service.
package connect

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"

	commonv1 "github.com/channel-manager/channel-manager/gen/go/common/v1"
	pmsv1 "github.com/channel-manager/channel-manager/gen/go/pms/v1"
	"github.com/channel-manager/channel-manager/gen/go/pms/v1/pmsv1connect"
	"github.com/channel-manager/channel-manager/services/pms/domain"
	"github.com/channel-manager/channel-manager/services/pms/usecases"
)

// Handler implements pmsv1connect.PmsServiceHandler.
type Handler struct {
	pmsv1connect.UnimplementedPmsServiceHandler
	svc *usecases.PmsService
}

// NewHandler creates a new handler.
func NewHandler(svc *usecases.PmsService) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) ListConnections(
	ctx context.Context,
	_ *connect.Request[pmsv1.ListConnectionsRequest],
) (*connect.Response[pmsv1.ListConnectionsResponse], error) {
	conns, err := h.svc.ListConnections(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*pmsv1.PmsConnection, 0, len(conns))
	for _, c := range conns {
		out = append(out, connectionToProto(c))
	}
	return connect.NewResponse(&pmsv1.ListConnectionsResponse{Connections: out}), nil
}

func (h *Handler) ConnectPms(
	ctx context.Context,
	req *connect.Request[pmsv1.ConnectPmsRequest],
) (*connect.Response[pmsv1.ConnectPmsResponse], error) {
	r := req.Msg
	if r.GetKind() == pmsv1.PmsKind_PMS_KIND_UNSPECIFIED {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("kind is required"))
	}
	conn, err := h.svc.ConnectPms(ctx, pmsKindToProvider(r.GetKind()), r.GetLabel(), r.GetCredentials())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&pmsv1.ConnectPmsResponse{
		Connection: connectionToProto(conn),
	}), nil
}

func (h *Handler) DisconnectPms(
	ctx context.Context,
	req *connect.Request[pmsv1.DisconnectPmsRequest],
) (*connect.Response[pmsv1.DisconnectPmsResponse], error) {
	if req.Msg.GetId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is required"))
	}
	conn, err := h.svc.DisconnectPms(ctx, req.Msg.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&pmsv1.DisconnectPmsResponse{Connection: connectionToProto(conn)}), nil
}

func (h *Handler) ListProperties(
	ctx context.Context,
	req *connect.Request[pmsv1.ListPropertiesRequest],
) (*connect.Response[pmsv1.ListPropertiesResponse], error) {
	props, err := h.svc.ListProperties(ctx, req.Msg.GetPmsId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*pmsv1.Property, 0, len(props))
	for _, p := range props {
		out = append(out, propertyToProto(p))
	}
	return connect.NewResponse(&pmsv1.ListPropertiesResponse{Properties: out}), nil
}

func (h *Handler) GetProperty(
	ctx context.Context,
	req *connect.Request[pmsv1.GetPropertyRequest],
) (*connect.Response[pmsv1.GetPropertyResponse], error) {
	if req.Msg.GetId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is required"))
	}
	prop, rts, err := h.svc.GetProperty(ctx, req.Msg.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	rtProto := make([]*pmsv1.RoomType, 0, len(rts))
	for _, rt := range rts {
		rtProto = append(rtProto, roomTypeToProto(rt))
	}
	return connect.NewResponse(&pmsv1.GetPropertyResponse{
		Property:  propertyToProto(prop),
		RoomTypes: rtProto,
	}), nil
}

func (h *Handler) ListRoomTypes(
	ctx context.Context,
	req *connect.Request[pmsv1.ListRoomTypesRequest],
) (*connect.Response[pmsv1.ListRoomTypesResponse], error) {
	if req.Msg.GetPropertyId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("property_id is required"))
	}
	rts, err := h.svc.ListRoomTypes(ctx, req.Msg.GetPropertyId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*pmsv1.RoomType, 0, len(rts))
	for _, rt := range rts {
		out = append(out, roomTypeToProto(rt))
	}
	return connect.NewResponse(&pmsv1.ListRoomTypesResponse{RoomTypes: out}), nil
}

func (h *Handler) SyncCatalog(
	ctx context.Context,
	req *connect.Request[pmsv1.SyncCatalogRequest],
) (*connect.Response[pmsv1.SyncCatalogResponse], error) {
	if req.Msg.GetConnectionId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("connection_id is required"))
	}
	result, err := h.svc.SyncCatalog(ctx, req.Msg.GetConnectionId(), domain.PropertySearchFilter{
		City:    req.Msg.GetCity(),
		Country: req.Msg.GetCountry(),
		Name:    req.Msg.GetName(),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&pmsv1.SyncCatalogResponse{
		PropertiesSynced: int32(result.PropertiesSynced), //nolint:gosec
		RoomTypesSynced:  int32(result.RoomTypesSynced),  //nolint:gosec
	}), nil
}

func (h *Handler) IngestAvailability(
	ctx context.Context,
	req *connect.Request[pmsv1.IngestAvailabilityRequest],
) (*connect.Response[pmsv1.IngestAvailabilityResponse], error) {
	r := req.Msg
	if r.GetPropertyId() == "" || r.GetCheckin() == nil || r.GetCheckout() == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("property_id, checkin, and checkout are required"))
	}
	checkin := calendarDateToTime(r.GetCheckin())
	checkout := calendarDateToTime(r.GetCheckout())
	adults := int(r.GetAdults())
	if adults <= 0 {
		adults = 1
	}
	rooms := int(r.GetRooms())
	if rooms <= 0 {
		rooms = 1
	}
	result, err := h.svc.IngestAvailability(ctx, r.GetPropertyId(), domain.AvailabilityQuery{
		Checkin:      checkin,
		Checkout:     checkout,
		Adults:       adults,
		Children:     int(r.GetChildren()),
		Rooms:        rooms,
		RoomTypeName: r.GetRoomTypeName(),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&pmsv1.IngestAvailabilityResponse{
		InventoryRowsAffected: result.InventoryRowsAffected,
		EventId:               result.EventID,
	}), nil
}

func (h *Handler) OrgHealth(
	ctx context.Context,
	req *connect.Request[pmsv1.OrgHealthRequest],
) (*connect.Response[pmsv1.OrgHealthResponse], error) {
	if req.Msg.GetConnectionId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("connection_id is required"))
	}
	health, err := h.svc.OrgHealth(ctx, req.Msg.GetConnectionId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&pmsv1.OrgHealthResponse{
		Status:           health.Status,
		Service:          health.Service,
		OrganizationId:   health.OrganizationID,
		AvailableActions: health.AvailableActions,
	}), nil
}

func (h *Handler) PropertyHealth(
	ctx context.Context,
	req *connect.Request[pmsv1.PropertyHealthRequest],
) (*connect.Response[pmsv1.PropertyHealthResponse], error) {
	if req.Msg.GetPropertyId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("property_id is required"))
	}
	health, err := h.svc.PropertyHealth(ctx, req.Msg.GetPropertyId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&pmsv1.PropertyHealthResponse{
		Status:           health.Status,
		Service:          health.Service,
		Property:         propertyToProto(health.Property),
		AvailableActions: health.AvailableActions,
	}), nil
}

func (h *Handler) GetQuote(
	ctx context.Context,
	req *connect.Request[pmsv1.GetQuoteRequest],
) (*connect.Response[pmsv1.GetQuoteResponse], error) {
	r := req.Msg
	if r.GetPropertyId() == "" || r.GetRoomId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("property_id and room_id are required"))
	}
	q, err := h.svc.GetQuote(ctx, r.GetPropertyId(), domain.QuoteQuery{
		RoomIDs:  []string{r.GetRoomId()},
		Checkin:  calendarDateToTime(r.GetCheckin()),
		Checkout: calendarDateToTime(r.GetCheckout()),
		Adults:   int(r.GetAdults()),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(quoteToProto(q)), nil
}

func (h *Handler) CreateBooking(
	ctx context.Context,
	req *connect.Request[pmsv1.CreateBookingRequest],
) (*connect.Response[pmsv1.CreateBookingResponse], error) {
	r := req.Msg
	if r.GetPropertyId() == "" || r.GetRoomId() == "" || r.GetGuestName() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("property_id, room_id, and guest_name are required"))
	}
	b, err := h.svc.CreateBooking(ctx, r.GetPropertyId(), domain.CreateBookingInput{
		RoomIDs:   []string{r.GetRoomId()},
		Checkin:   calendarDateToTime(r.GetCheckin()),
		Checkout:  calendarDateToTime(r.GetCheckout()),
		GuestName: r.GetGuestName(),
		Email:     r.GetEmail(),
		Phone:     r.GetPhone(),
		Adults:    int(r.GetAdults()),
		Children:  int(r.GetChildren()),
		Notes:     r.GetNotes(),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&pmsv1.CreateBookingResponse{Booking: bookingToProto(b)}), nil
}

func (h *Handler) GetBooking(
	ctx context.Context,
	req *connect.Request[pmsv1.GetBookingRequest],
) (*connect.Response[pmsv1.GetBookingResponse], error) {
	r := req.Msg
	if r.GetPropertyId() == "" || r.GetBookingId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("property_id and booking_id are required"))
	}
	b, err := h.svc.GetBooking(ctx, r.GetPropertyId(), domain.GetBookingInput{
		BookingID: r.GetBookingId(),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&pmsv1.GetBookingResponse{Booking: bookingToProto(b)}), nil
}

func (h *Handler) UpdateBooking(
	ctx context.Context,
	req *connect.Request[pmsv1.UpdateBookingRequest],
) (*connect.Response[pmsv1.UpdateBookingResponse], error) {
	r := req.Msg
	if r.GetPropertyId() == "" || r.GetBookingId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("property_id and booking_id are required"))
	}
	in := domain.UpdateBookingInput{
		BookingID: r.GetBookingId(),
		GuestName: r.GetGuestName(),
		Email:     r.GetEmail(),
		Phone:     r.GetPhone(),
		Notes:     r.GetNotes(),
	}
	if roomID := r.GetRoomId(); roomID != "" {
		in.RoomIDs = []string{roomID}
	}
	if r.GetCheckin() != nil {
		t := calendarDateToTime(r.GetCheckin())
		in.Checkin = &t
	}
	if r.GetCheckout() != nil {
		t := calendarDateToTime(r.GetCheckout())
		in.Checkout = &t
	}
	if r.Adults != nil {
		a := int(*r.Adults)
		in.Adults = &a
	}
	if r.Children != nil {
		c := int(*r.Children)
		in.Children = &c
	}
	b, err := h.svc.UpdateBooking(ctx, r.GetPropertyId(), in)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&pmsv1.UpdateBookingResponse{Booking: bookingToProto(b)}), nil
}

func (h *Handler) CancelBooking(
	ctx context.Context,
	req *connect.Request[pmsv1.CancelBookingRequest],
) (*connect.Response[pmsv1.CancelBookingResponse], error) {
	r := req.Msg
	if r.GetPropertyId() == "" || r.GetBookingId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("property_id and booking_id are required"))
	}
	result, err := h.svc.CancelBooking(ctx, r.GetPropertyId(), domain.CancelBookingInput{
		BookingID: r.GetBookingId(),
		Reason:    r.GetReason(),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&pmsv1.CancelBookingResponse{
		BookingId: result.BookingID,
		Status:    result.Status,
		Message:   result.Message,
	}), nil
}

func (h *Handler) DeleteBooking(
	ctx context.Context,
	req *connect.Request[pmsv1.DeleteBookingRequest],
) (*connect.Response[pmsv1.DeleteBookingResponse], error) {
	r := req.Msg
	if r.GetPropertyId() == "" || r.GetBookingId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("property_id and booking_id are required"))
	}
	result, err := h.svc.DeleteBooking(ctx, r.GetPropertyId(), r.GetBookingId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&pmsv1.DeleteBookingResponse{
		BookingId: result.BookingID,
		Status:    result.Status,
	}), nil
}

func (h *Handler) ListBookings(
	ctx context.Context,
	req *connect.Request[pmsv1.ListBookingsRequest],
) (*connect.Response[pmsv1.ListBookingsResponse], error) {
	r := req.Msg
	if r.GetPropertyId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("property_id is required"))
	}

	in := domain.ListBookingsInput{
		Status: r.GetStatus(),
	}
	if r.GetStartDate() != nil {
		t := calendarDateToTime(r.GetStartDate())
		in.StartDate = &t
	}
	if r.GetEndDate() != nil {
		t := calendarDateToTime(r.GetEndDate())
		in.EndDate = &t
	}

	result, err := h.svc.ListBookings(ctx, r.GetPropertyId(), in)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	out := make([]*pmsv1.PmsBooking, 0, len(result.Bookings))
	for _, b := range result.Bookings {
		// Use a local copy of b so we can take its address safely in old Go versions,
		// though in 1.22+ it's fine.
		bCopy := b
		out = append(out, bookingToProto(&bCopy))
	}

	return connect.NewResponse(&pmsv1.ListBookingsResponse{
		Bookings: out,
		Count:    int32(result.Count),
	}), nil
}

func calendarDateToTime(cd *commonv1.CalendarDate) time.Time {
	if cd == nil {
		return time.Time{}
	}
	return time.Date(int(cd.GetYear()), time.Month(cd.GetMonth()), int(cd.GetDay()), 0, 0, 0, 0, time.UTC)
}
