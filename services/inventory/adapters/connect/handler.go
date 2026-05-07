// Package connect implements the Connect-RPC handler for the inventory service.
package connect

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	commonv1 "github.com/channel-manager/channel-manager/gen/go/common/v1"
	inventoryv1 "github.com/channel-manager/channel-manager/gen/go/inventory/v1"
	"github.com/channel-manager/channel-manager/gen/go/inventory/v1/inventoryv1connect"
	"github.com/channel-manager/channel-manager/services/inventory/domain"
	"github.com/channel-manager/channel-manager/services/inventory/usecases"
)

// Handler implements inventoryv1connect.InventoryServiceHandler.
type Handler struct {
	inventoryv1connect.UnimplementedInventoryServiceHandler
	svc *usecases.InventoryService
}

// NewHandler creates a new Connect-RPC handler wrapping the given service.
func NewHandler(svc *usecases.InventoryService) *Handler {
	return &Handler{svc: svc}
}

// GetInventory returns inventory days for a room type within a date range.
func (h *Handler) GetInventory(
	ctx context.Context,
	req *connect.Request[inventoryv1.GetInventoryRequest],
) (*connect.Response[inventoryv1.GetInventoryResponse], error) {
	r := req.Msg
	if r.GetRange() == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("range is required"))
	}
	if r.GetRoomTypeId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("room_type_id is required"))
	}

	from := calendarDateToTime(r.GetRange().GetStart())
	// DateRange is [start, end) — subtract one day so the repo query is inclusive [from, to].
	to := calendarDateToTime(r.GetRange().GetEnd()).AddDate(0, 0, -1)

	days, err := h.svc.GetInventory(ctx, usecases.GetInventoryInput{
		RoomTypeID: r.GetRoomTypeId(),
		From:       from,
		To:         to,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp := &inventoryv1.GetInventoryResponse{
		Days: make([]*inventoryv1.InventoryDay, 0, len(days)),
	}
	for _, d := range days {
		resp.Days = append(resp.Days, domainToProto(d))
	}
	return connect.NewResponse(resp), nil
}

// BulkUpsertInventory atomically writes inventory days with idempotency.
func (h *Handler) BulkUpsertInventory(
	ctx context.Context,
	req *connect.Request[inventoryv1.BulkUpsertInventoryRequest],
) (*connect.Response[inventoryv1.BulkUpsertInventoryResponse], error) {
	r := req.Msg
	if len(r.GetDays()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("days must not be empty"))
	}

	days := make([]domain.InventoryDay, 0, len(r.GetDays()))
	for _, pd := range r.GetDays() {
		days = append(days, protoToDomain(pd))
	}

	result, err := h.svc.BulkUpsertInventory(ctx, usecases.BulkUpsertInput{
		Days:           days,
		IdempotencyKey: r.GetIdempotencyKey().GetKey(),
	})
	if err != nil {
		if errors.Is(err, usecases.ErrDuplicateRequest) {
			return nil, connect.NewError(connect.CodeAlreadyExists, errors.New("duplicate request: idempotency key already processed"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&inventoryv1.BulkUpsertInventoryResponse{
		RowsAffected: result.RowsAffected,
		EventId:      result.EventID,
	}), nil
}

// calendarDateToTime converts a proto CalendarDate to a UTC time.Time (midnight).
func calendarDateToTime(cd *commonv1.CalendarDate) time.Time {
	if cd == nil {
		return time.Time{}
	}
	return time.Date(int(cd.GetYear()), time.Month(cd.GetMonth()), int(cd.GetDay()), 0, 0, 0, 0, time.UTC)
}

// protoToDomain maps a proto InventoryDay to the domain model.
func protoToDomain(pd *inventoryv1.InventoryDay) domain.InventoryDay {
	d := domain.InventoryDay{
		RoomTypeID: pd.GetRoomTypeId(),
		StayDate:   calendarDateToTime(pd.GetDate()),
		Available:  int(pd.GetAvailable()),
		StopSell:   pd.GetStopSell(),
		CTA:        pd.GetCta(),
		CTD:        pd.GetCtd(),
	}
	if ms := pd.GetMinStay(); ms != 0 {
		v := int32(ms)
		d.MinStay = &v
	}
	if ms := pd.GetMaxStay(); ms != 0 {
		v := int32(ms)
		d.MaxStay = &v
	}
	return d
}

// domainToProto maps a domain InventoryDay to a proto message.
func domainToProto(d domain.InventoryDay) *inventoryv1.InventoryDay {
	pd := &inventoryv1.InventoryDay{
		RoomTypeId: d.RoomTypeID,
		Date: &commonv1.CalendarDate{
			Year:  int32(d.StayDate.Year()),
			Month: int32(d.StayDate.Month()),
			Day:   int32(d.StayDate.Day()),
		},
		Available: int32(d.Available), //nolint:gosec
		StopSell:  d.StopSell,
		Cta:       d.CTA,
		Ctd:       d.CTD,
		UpdatedAt: timestamppb.New(d.UpdatedAt),
	}
	if d.MinStay != nil {
		pd.MinStay = *d.MinStay
	}
	if d.MaxStay != nil {
		pd.MaxStay = *d.MaxStay
	}
	return pd
}
