package connect

import (
	"google.golang.org/protobuf/types/known/timestamppb"

	pmsv1 "github.com/channel-manager/channel-manager/gen/go/pms/v1"
	"github.com/channel-manager/channel-manager/services/pms/domain"
)

func pmsKindToProvider(k pmsv1.PmsKind) string {
	switch k {
	case pmsv1.PmsKind_PMS_KIND_CLOUDBEDS:
		return "cloudbeds"
	case pmsv1.PmsKind_PMS_KIND_MEWS:
		return "mews"
	case pmsv1.PmsKind_PMS_KIND_OPERA:
		return "opera"
	case pmsv1.PmsKind_PMS_KIND_MYPMS:
		return "mypms"
	case pmsv1.PmsKind_PMS_KIND_CUSTOM:
		return "custom"
	default:
		return "mypms"
	}
}

func connectionToProto(c domain.Connection) *pmsv1.PmsConnection {
	out := &pmsv1.PmsConnection{
		Id:     c.ID,
		Kind:   providerToKind(c.Provider),
		Status: statusToProto(c.Status),
		Label:  c.Name,
	}
	if c.LastSyncAt != nil {
		out.LastSyncAt = timestamppb.New(*c.LastSyncAt)
	}
	if c.LastError != "" {
		out.LastError = c.LastError
	}
	if !c.CreatedAt.IsZero() {
		out.CreatedAt = timestamppb.New(c.CreatedAt)
	}
	if !c.UpdatedAt.IsZero() {
		out.UpdatedAt = timestamppb.New(c.UpdatedAt)
	}
	return out
}

func providerToKind(provider string) pmsv1.PmsKind {
	switch provider {
	case "cloudbeds":
		return pmsv1.PmsKind_PMS_KIND_CLOUDBEDS
	case "mews":
		return pmsv1.PmsKind_PMS_KIND_MEWS
	case "opera":
		return pmsv1.PmsKind_PMS_KIND_OPERA
	case "mypms":
		return pmsv1.PmsKind_PMS_KIND_MYPMS
	default:
		return pmsv1.PmsKind_PMS_KIND_CUSTOM
	}
}

func statusToProto(status string) pmsv1.PmsStatus {
	switch status {
	case "active":
		return pmsv1.PmsStatus_PMS_STATUS_ACTIVE
	case "paused":
		return pmsv1.PmsStatus_PMS_STATUS_PAUSED
	case "disabled", "disconnected":
		return pmsv1.PmsStatus_PMS_STATUS_DISCONNECTED
	case "error":
		return pmsv1.PmsStatus_PMS_STATUS_ERROR
	default:
		return pmsv1.PmsStatus_PMS_STATUS_UNSPECIFIED
	}
}

func propertyToProto(p domain.Property) *pmsv1.Property {
	out := &pmsv1.Property{
		Id:               p.ID,
		PmsId:            p.ConnectionID,
		ExternalPropertyId: p.ExternalID,
		Name:             p.Name,
		Timezone:         p.Timezone,
		DefaultCurrency:  p.DefaultCurrency,
	}
	if !p.CreatedAt.IsZero() {
		out.CreatedAt = timestamppb.New(p.CreatedAt)
	}
	if !p.UpdatedAt.IsZero() {
		out.UpdatedAt = timestamppb.New(p.UpdatedAt)
	}
	return out
}

func roomTypeToProto(rt domain.RoomType) *pmsv1.RoomType {
	var protoRooms []*pmsv1.Room
	for _, r := range rt.Rooms {
		protoRoom := &pmsv1.Room{
			Id:         r.ID,
			PropertyId: r.PropertyID,
			RoomTypeId: r.RoomTypeID,
			ExternalId: r.ExternalID,
			Name:       r.Name,
			IsActive:   r.IsActive,
		}
		if !r.CreatedAt.IsZero() {
			protoRoom.CreatedAt = timestamppb.New(r.CreatedAt)
		}
		if !r.UpdatedAt.IsZero() {
			protoRoom.UpdatedAt = timestamppb.New(r.UpdatedAt)
		}
		protoRooms = append(protoRooms, protoRoom)
	}

	out := &pmsv1.RoomType{
		Id:           rt.ID,
		PropertyId:   rt.PropertyID,
		Code:         rt.Code,
		Name:         rt.Name,
		MaxOccupancy: int32(rt.MaxOccupancy),  //nolint:gosec
		BaseOccupancy: int32(rt.BaseOccupancy), //nolint:gosec
		Rooms:        protoRooms,
	}
	if !rt.CreatedAt.IsZero() {
		out.CreatedAt = timestamppb.New(rt.CreatedAt)
	}
	if !rt.UpdatedAt.IsZero() {
		out.UpdatedAt = timestamppb.New(rt.UpdatedAt)
	}
	return out
}

func quoteToProto(q *domain.Quote) *pmsv1.GetQuoteResponse {
	if q == nil {
		return &pmsv1.GetQuoteResponse{}
	}
	return &pmsv1.GetQuoteResponse{
		RoomId:        q.RoomID,
		RoomName:      q.RoomName,
		RoomType:      q.RoomType,
		Nights:        int32(q.Nights), //nolint:gosec
		Adults:        int32(q.Adults), //nolint:gosec
		Capacity:      int32(q.Capacity), //nolint:gosec
		PricePerNight: q.PricePerNight,
		TotalPrice:    q.TotalPrice,
		Currency:      q.Currency,
		IsAvailable:   q.IsAvailable,
	}
}

func bookingToProto(b *domain.PmsBooking) *pmsv1.PmsBooking {
	if b == nil {
		return nil
	}
	return &pmsv1.PmsBooking{
		BookingId:     b.BookingID,
		Status:        b.Status,
		GuestName:     b.GuestName,
		Email:         b.Email,
		Phone:         b.Phone,
		RoomId:        b.RoomID,
		RoomName:      b.RoomName,
		RoomType:      b.RoomType,
		PropertyName:  b.PropertyName,
		Checkin:       b.Checkin,
		Checkout:      b.Checkout,
		Adults:        int32(b.Adults),   //nolint:gosec
		Children:      int32(b.Children), //nolint:gosec
		Notes:         b.Notes,
		PaymentStatus: b.PaymentStatus,
		Source:        b.Source,
		Message:       b.Message,
	}
}
