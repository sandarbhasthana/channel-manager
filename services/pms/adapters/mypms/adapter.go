package mypms

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/channel-manager/channel-manager/services/pms/domain"
	"github.com/channel-manager/channel-manager/services/pms/ports"
)

// Adapter implements ports.BookingEngineClient for the MyPMS webhook API.
type Adapter struct {
	client *Client
}

// NewAdapter creates an adapter backed by the given client.
func NewAdapter(client *Client) *Adapter {
	return &Adapter{client: client}
}

// NewAdapterFromConfig builds a client from base URL and bearer token.
func NewAdapterFromConfig(baseURL, bearerToken string) *Adapter {
	return NewAdapter(NewClient(Config{BaseURL: baseURL, Token: bearerToken}))
}

func (a *Adapter) PmsID() string { return "mypms" }

func (a *Adapter) Capabilities() []domain.PmsCapability {
	return []domain.PmsCapability{
		domain.CapabilityListProperties,
		domain.CapabilityListRoomTypes,
		domain.CapabilityGetInventory,
		domain.CapabilitySearchAvailability,
		domain.CapabilitySearchFlexibleAvailability,
		domain.CapabilityGetQuote,
		domain.CapabilityCreateBooking,
		domain.CapabilityGetBooking,
		domain.CapabilityUpdateBooking,
		domain.CapabilityCancelBooking,
	}
}

// Ensure Adapter implements BookingEngineClient.
var _ ports.BookingEngineClient = (*Adapter)(nil)

func (a *Adapter) OrgHealth(ctx context.Context) (*domain.OrgHealth, error) {
	resp, err := a.client.OrgHealth(ctx)
	if err != nil {
		return nil, err
	}
	return &domain.OrgHealth{
		Status:           resp.Status,
		Service:          resp.Service,
		OrganizationID:   resp.OrganizationID,
		AvailableActions: resp.AvailableActions,
	}, nil
}

func (a *Adapter) SearchProperties(ctx context.Context, filter domain.PropertySearchFilter) ([]domain.Property, error) {
	resp, err := a.client.SearchProperties(ctx, filter.City, filter.Country, filter.Name)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Property, 0, len(resp.Properties))
	for _, p := range resp.Properties {
		out = append(out, domain.Property{
			ExternalID:      p.PropertyID,
			Name:            p.Name,
			DefaultCurrency: p.Currency,
			City:            p.City,
			Country:         p.Country,
		})
	}
	return out, nil
}

func (a *Adapter) PropertyHealth(ctx context.Context, externalPropertyID string) (*domain.PropertyHealth, error) {
	resp, err := a.client.PropertyHealth(ctx, externalPropertyID)
	if err != nil {
		return nil, err
	}
	return &domain.PropertyHealth{
		Status:  resp.Status,
		Service: resp.Service,
		Property: domain.Property{
			ExternalID:      resp.Property.PropertyID,
			Name:            resp.Property.Name,
			DefaultCurrency: resp.Property.Currency,
			City:            resp.Property.City,
			Country:         resp.Property.Country,
		},
		AvailableActions: resp.AvailableActions,
	}, nil
}

func (a *Adapter) ListRoomTypes(ctx context.Context, externalPropertyID string) ([]domain.RoomType, error) {
	resp, err := a.client.GetRoomDetails(ctx, externalPropertyID, "", "")
	if err != nil {
		return nil, err
	}
	return roomTypesFromDetails(externalPropertyID, resp), nil
}

func roomTypesFromDetails(propertyID string, resp *GetRoomDetailsResponse) []domain.RoomType {
	details := resp.RoomTypesList()
	out := make([]domain.RoomType, 0, len(details))
	for _, rt := range details {
		extID := rt.RoomTypeID
		if extID == "" {
			extID = rt.ID
		}
		if extID == "" {
			extID = rt.RoomType
		}
		name := rt.Name
		if name == "" {
			name = rt.RoomType
		}
		maxOcc := rt.MaxOccupancy
		if maxOcc == 0 {
			maxOcc = rt.Capacity
		}
		baseOcc := rt.BaseOccupancy
		if baseOcc == 0 {
			baseOcc = maxOcc
		}

		var rooms []domain.Room
		// If rooms are provided nested inside the room type
		for _, r := range rt.Rooms {
			rooms = append(rooms, domain.Room{
				ExternalID: r.GetID(),
				Name:       r.Name,
				IsActive:   true,
			})
		}
		// If rooms are provided globally
		for _, r := range resp.RoomsList() {
			if r.RoomTypeID == extID || r.RoomTypeID == rt.RoomType {
				rooms = append(rooms, domain.Room{
					ExternalID: r.GetID(),
					Name:       r.Name,
					IsActive:   true,
				})
			}
		}

		out = append(out, domain.RoomType{
			ExternalPropertyID: propertyID,
			ExternalID:         extID,
			Code:               extID,
			Name:               name,
			MaxOccupancy:       maxOcc,
			BaseOccupancy:      baseOcc,
			Rooms:              rooms,
		})
	}
	return out
}

func (a *Adapter) SearchAvailability(ctx context.Context, externalPropertyID string, q domain.AvailabilityQuery) ([]domain.AvailabilityOffer, error) {
	resp, err := a.client.SearchAvailability(ctx, externalPropertyID, SearchAvailabilityRequest{
		Checkin:  q.Checkin.Format("2006-01-02"),
		Checkout: q.Checkout.Format("2006-01-02"),
		Adults:   q.Adults,
		Children: q.Children,
		Rooms:    q.Rooms,
		RoomType: q.RoomTypeName,
	})
	if err != nil {
		return nil, err
	}
	rooms := resp.RoomsList()
	out := make([]domain.AvailabilityOffer, 0, len(rooms))
	for _, r := range rooms {
		out = append(out, offerFromRoom(r))
	}
	return out, nil
}

func (a *Adapter) SearchFlexibleAvailability(ctx context.Context, externalPropertyID string, q domain.FlexibleAvailabilityQuery) (*domain.FlexibleAvailabilityResult, error) {
	resp, err := a.client.SearchFlexibleAvailability(ctx, externalPropertyID, SearchFlexibleAvailabilityRequest{
		Nights:          q.Nights,
		Adults:          q.Adults,
		Children:        q.Children,
		Rooms:           q.Rooms,
		RoomType:        q.RoomTypeName,
		EarliestCheckin: q.EarliestCheckin,
		LatestCheckout:  q.LatestCheckout,
		Limit:           q.Limit,
		SortBy:          q.SortBy,
	})
	if err != nil {
		return nil, err
	}
	stays := resp.StaysList()
	result := &domain.FlexibleAvailabilityResult{
		PropertyID:      resp.Property.PropertyID,
		PropertyName:    resp.Property.Name,
		Nights:          resp.Nights,
		Adults:          resp.Adults,
		Children:        resp.Children,
		RequestedRooms:  resp.RequestedRooms,
		SortBy:          resp.SortBy,
		EarliestCheckin: resp.SearchWindow.EarliestCheckin,
		LatestCheckout:  resp.SearchWindow.LatestCheckout,
		TotalMatching:   resp.TotalMatching,
		Returned:        resp.Returned,
		Stays:           make([]domain.FlexibleStay, 0, len(stays)),
	}
	if result.Nights == 0 {
		result.Nights = q.Nights
	}
	if result.Adults == 0 {
		result.Adults = q.Adults
	}
	result.Children = resp.Children
	if result.RequestedRooms == 0 {
		result.RequestedRooms = q.Rooms
	}
	for _, stay := range stays {
		offers := make([]domain.AvailabilityOffer, 0, len(stay.AvailableRooms))
		for _, room := range stay.AvailableRooms {
			offers = append(offers, offerFromRoom(room))
		}
		mapped := domain.FlexibleStay{
			Checkin:        stay.Checkin,
			Checkout:       stay.Checkout,
			Nights:         stay.Nights,
			CanAccommodate: stay.CanAccommodate,
			RoomTypes:      append([]string(nil), stay.MatchingRoomTypes...),
			TotalAvailable: stay.TotalAvailable,
			Offers:         offers,
		}
		if stay.StartingRate != nil {
			mapped.StartingRate = &domain.FlexibleStayRate{
				PerNight: stay.StartingRate.PerNight,
				Total:    stay.StartingRate.Total,
				Currency: stay.StartingRate.Currency,
			}
		}
		if mapped.Nights == 0 {
			mapped.Nights = q.Nights
		}
		if mapped.TotalAvailable == 0 {
			mapped.TotalAvailable = len(offers)
		}
		result.Stays = append(result.Stays, mapped)
	}
	if result.Returned == 0 {
		result.Returned = len(result.Stays)
	}
	return result, nil
}

func offerFromRoom(r AvailabilityRoom) domain.AvailabilityOffer {
	rtID := r.RoomTypeID
	if rtID == "" {
		rtID = r.RoomType
	}
	roomIDs := r.NormalizedRoomIDs()
	roomCount := r.NormalizedRoomCount()
	avail := r.AvailableUnits
	if avail <= 0 {
		avail = r.AvailableQty
	}
	if avail <= 0 {
		avail = 1
	}
	if roomCount < 1 {
		roomCount = len(roomIDs)
	}
	return domain.AvailabilityOffer{
		RoomIDs:        roomIDs,
		RoomCount:      roomCount,
		RoomNames:      r.NormalizedRoomNames(),
		RoomTypes:      append([]string(nil), r.RoomTypes...),
		RoomTypeID:     rtID,
		RoomTypeName:   firstNonEmpty(r.RoomTypeName, r.RoomType),
		AvailableUnits: avail,
		IsAvailable:    true,
		PricePerNight:  r.PricePerNight,
		TotalPrice:     r.TotalPrice,
		Currency:       r.Currency,
		Capacity:       r.Capacity,
		MaxAdults:      r.MaxAdults,
		MaxChildren:    r.MaxChildren,
		Description:    r.Description,
		Amenities:      append([]string(nil), r.Amenities...),
	}
}

func (a *Adapter) GetInventory(ctx context.Context, externalPropertyID, roomTypeID string, from, to time.Time) ([]domain.InventorySnapshot, error) {
	q := domain.AvailabilityQuery{
		Checkin:  from,
		Checkout: to.AddDate(0, 0, 1),
		Adults:   1,
		Rooms:    1,
	}
	offers, err := a.SearchAvailability(ctx, externalPropertyID, q)
	if err != nil {
		return nil, err
	}
	// search_availability returns stay-level offers; synthesize per-day snapshots.
	snapshots := make([]domain.InventorySnapshot, 0)
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		for _, o := range offers {
			if roomTypeID != "" && o.RoomTypeID != roomTypeID {
				continue
			}
			avail := o.AvailableUnits
			if !o.IsAvailable {
				avail = 0
			}
			snapshots = append(snapshots, domain.InventorySnapshot{
				ExternalRoomTypeID: o.RoomTypeID,
				Date:               d,
				Available:          avail,
			})
		}
	}
	return snapshots, nil
}

func (a *Adapter) GetQuote(ctx context.Context, externalPropertyID string, q domain.QuoteQuery) (*domain.Quote, error) {
	resp, err := a.client.GetQuote(ctx, externalPropertyID, GetQuoteRequest{
		RoomIDs:  append([]string(nil), q.RoomIDs...),
		Checkin:  q.Checkin.Format("2006-01-02"),
		Checkout: q.Checkout.Format("2006-01-02"),
		Adults:   q.Adults,
		Children: q.Children,
	})
	if err != nil {
		return nil, err
	}
	return quoteToDomain(resp), nil
}

func (a *Adapter) CreateBooking(ctx context.Context, externalPropertyID string, in domain.CreateBookingInput) (*domain.PmsBooking, error) {
	resp, err := a.client.CreateBooking(ctx, externalPropertyID, CreateBookingRequest{
		RoomIDs:        append([]string(nil), in.RoomIDs...),
		Checkin:        in.Checkin.Format("2006-01-02"),
		Checkout:       in.Checkout.Format("2006-01-02"),
		GuestName:      in.GuestName,
		Email:          in.Email,
		Phone:          in.Phone,
		Adults:         in.Adults,
		Children:       in.Children,
		Notes:          in.Notes,
		TotalAmount:    in.TotalAmount,
		Currency:       in.Currency,
		IdempotencyKey: in.IdempotencyKey,
	})
	if err != nil {
		return nil, err
	}
	if len(resp.BookingIDs) != 1 || len(resp.RoomIDs) == 0 {
		return nil, fmt.Errorf("mypms: create booking must return one confirmation number")
	}
	return &domain.PmsBooking{
		BookingIDs:    append([]string(nil), resp.BookingIDs...),
		RoomIDs:       append([]string(nil), resp.RoomIDs...),
		RoomNames:     append([]string(nil), resp.RoomNames...),
		RoomTypes:     append([]string(nil), resp.RoomTypes...),
		GroupStatus:   resp.GroupStatus,
		Status:        resp.GroupStatus,
		GuestName:     resp.GuestName,
		PropertyName:  resp.PropertyName,
		Checkin:       resp.Checkin,
		Checkout:      resp.Checkout,
		Adults:        resp.Adults,
		Children:      resp.Children,
		PaymentStatus: resp.PaymentStatus,
		Message:       resp.Message,
	}, nil
}

func (a *Adapter) GetBooking(ctx context.Context, externalPropertyID string, in domain.GetBookingInput) (*domain.PmsBooking, error) {
	resp, err := a.client.GetBooking(ctx, externalPropertyID, GetBookingRequest{
		BookingID:        in.BookingID,
		GuestSurname:     in.GuestSurname,
		GuestFirstName:   in.GuestFirstName,
		GuestName:        in.GuestName,
		Phone:            in.Phone,
		Email:            in.Email,
		Checkin:          in.Checkin,
		PhoneMatchLast10: in.PhoneMatchLast10,
	})
	if err != nil {
		return nil, err
	}
	return bookingToDomain(resp), nil
}

func (a *Adapter) UpdateBooking(ctx context.Context, externalPropertyID string, in domain.UpdateBookingInput) (*domain.PmsBooking, error) {
	req := UpdateBookingRequest{BookingID: in.BookingID, GuestSurname: in.GuestSurname}
	if in.Checkin != nil {
		s := in.Checkin.Format("2006-01-02")
		req.Checkin = s
	}
	if in.Checkout != nil {
		s := in.Checkout.Format("2006-01-02")
		req.Checkout = s
	}
	req.GuestName = in.GuestName
	req.Email = in.Email
	req.Phone = in.Phone
	req.Adults = in.Adults
	req.Children = in.Children
	req.Notes = in.Notes
	req.RoomIDs = append([]string(nil), in.RoomIDs...)
	resp, err := a.client.UpdateBooking(ctx, externalPropertyID, req)
	if err != nil {
		return nil, err
	}
	return bookingToDomain(resp), nil
}

func (a *Adapter) CancelBooking(ctx context.Context, externalPropertyID string, in domain.CancelBookingInput) (*domain.CancelBookingResult, error) {
	resp, err := a.client.CancelBooking(ctx, externalPropertyID, CancelBookingRequest{
		BookingID:    in.BookingID,
		GuestSurname: in.GuestSurname,
		Reason:       in.Reason,
	})
	if err != nil {
		return nil, err
	}
	return &domain.CancelBookingResult{
		BookingID: resp.BookingID,
		Status:    resp.Status,
		Message:   resp.Message,
	}, nil
}

func (a *Adapter) DeleteBooking(ctx context.Context, externalPropertyID, bookingID string) (*domain.DeleteBookingResult, error) {
	resp, err := a.client.DeleteBooking(ctx, externalPropertyID, DeleteBookingRequest{
		BookingID: bookingID,
	})
	if err != nil {
		return nil, err
	}
	return &domain.DeleteBookingResult{
		BookingID: resp.BookingID,
		Status:    resp.Status,
		Message:   resp.Message,
	}, nil
}

func (a *Adapter) ListBookings(ctx context.Context, externalPropertyID string, in domain.ListBookingsInput) (*domain.ListBookingsResult, error) {
	req := ListBookingsRequest{
		Status: in.Status,
	}
	if in.StartDate != nil {
		req.StartDate = in.StartDate.Format("2006-01-02")
	}
	if in.EndDate != nil {
		req.EndDate = in.EndDate.Format("2006-01-02")
	}

	resp, err := a.client.ListBookings(ctx, externalPropertyID, req)
	if err != nil {
		return nil, err
	}

	var bookings []domain.PmsBooking
	for _, b := range resp.Data.Bookings {
		bookings = append(bookings, *bookingToDomain(&b))
	}

	return &domain.ListBookingsResult{
		Bookings: bookings,
		Count:    resp.Data.Count,
	}, nil
}

func quoteToDomain(q *Quote) *domain.Quote {
	if q == nil {
		return nil
	}
	ids := append([]string(nil), q.RoomIDs...)
	if len(ids) == 0 && strings.TrimSpace(q.RoomID) != "" {
		ids = []string{strings.TrimSpace(q.RoomID)}
	}
	return &domain.Quote{
		RoomIDs:       ids,
		RoomName:      q.RoomName,
		RoomType:      q.RoomType,
		Nights:        q.Nights,
		Adults:        q.Adults,
		Capacity:      q.Capacity,
		PricePerNight: q.PricePerNight,
		TotalPrice:    q.TotalPrice,
		Currency:      q.Currency,
		IsAvailable:   q.IsAvailable,

		FirstNightPrice:   q.FirstNightPrice,
		RatePaymentType:   q.RatePaymentType,
		DepositPercent:    q.DepositPercent,
		UnavailableReason: q.UnavailableReason,
	}
}

func bookingToDomain(b *Booking) *domain.PmsBooking {
	if b == nil {
		return nil
	}
	ids := append([]string(nil), b.RoomIDs...)
	if len(ids) == 0 && strings.TrimSpace(b.RoomID) != "" {
		ids = []string{strings.TrimSpace(b.RoomID)}
	}
	return &domain.PmsBooking{
		BookingID:     b.BookingID,
		Status:        b.Status,
		GuestName:     b.GuestName,
		Email:         b.Email,
		Phone:         b.Phone,
		RoomIDs:       ids,
		RoomID:        firstNonEmpty(ids...),
		RoomName:      b.RoomName,
		RoomType:      b.RoomType,
		PropertyName:  b.PropertyName,
		Checkin:       b.Checkin,
		Checkout:      b.Checkout,
		Adults:        b.Adults,
		Children:      b.Children,
		Notes:         b.Notes,
		PaymentStatus: b.PaymentStatus,
		Source:        b.Source,
		Message:       b.Message,
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// CredentialsFromMap extracts base_url and bearer_token from a connection credential map.
func CredentialsFromMap(creds map[string]string) (baseURL, token string, err error) {
	baseURL = creds["base_url"]
	if baseURL == "" {
		baseURL = creds["baseUrl"]
	}
	token = creds["bearer_token"]
	if token == "" {
		token = creds["bearerToken"]
	}
	if token == "" {
		token = creds["token"]
	}
	if baseURL == "" || token == "" {
		return "", "", fmt.Errorf("mypms: credentials require base_url and bearer_token")
	}
	return baseURL, token, nil
}
func TestRoomTypesFromDetails(propertyID string, resp *GetRoomDetailsResponse) []domain.RoomType {
	return roomTypesFromDetails(propertyID, resp)
}
