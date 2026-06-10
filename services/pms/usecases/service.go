package usecases

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	platformauth "github.com/channel-manager/channel-manager/platform/auth"
	"github.com/channel-manager/channel-manager/services/pms/adapters/mypms"
	"github.com/channel-manager/channel-manager/services/pms/domain"
	"github.com/channel-manager/channel-manager/services/pms/ports"
)

// PmsService orchestrates PMS connections, catalog sync, and booking engine calls.
type PmsService struct {
	conns     ports.ConnectionRepository
	props     ports.PropertyRepository
	roomTypes ports.RoomTypeRepository
	rooms     ports.RoomRepository
	secrets   ports.SecretResolver
	inventory ports.InventoryWriter
	engines   map[string]func(map[string]string) (ports.BookingEngineClient, error)
	log       *slog.Logger
}

// NewPmsService creates a PmsService.
func NewPmsService(
	conns ports.ConnectionRepository,
	props ports.PropertyRepository,
	roomTypes ports.RoomTypeRepository,
	rooms ports.RoomRepository,
	secrets ports.SecretResolver,
	inventory ports.InventoryWriter,
) *PmsService {
	s := &PmsService{
		conns:     conns,
		props:     props,
		roomTypes: roomTypes,
		rooms:     rooms,
		secrets:   secrets,
		inventory: inventory,
		engines:   make(map[string]func(map[string]string) (ports.BookingEngineClient, error)),
		log:       slog.Default().With("service", "pms"),
	}
	s.engines["mypms"] = func(creds map[string]string) (ports.BookingEngineClient, error) {
		baseURL, token, err := mypms.CredentialsFromMap(creds)
		if err != nil {
			return nil, err
		}
		return mypms.NewAdapterFromConfig(baseURL, token), nil
	}
	return s
}

func (s *PmsService) engineForConnection(ctx context.Context, connectionID string) (ports.BookingEngineClient, domain.Connection, error) {
	conn, err := s.conns.GetByID(ctx, connectionID)
	if err != nil {
		return nil, domain.Connection{}, fmt.Errorf("pms: get connection: %w", err)
	}
	factory, ok := s.engines[conn.Provider]
	if !ok {
		return nil, conn, fmt.Errorf("pms: unsupported provider %q", conn.Provider)
	}
	creds, err := s.secrets.Resolve(ctx, conn.SecretRef)
	if err != nil {
		return nil, conn, fmt.Errorf("pms: resolve credentials: %w", err)
	}
	client, err := factory(creds)
	if err != nil {
		return nil, conn, err
	}
	return client, conn, nil
}

// ConnectPms stores a new PMS connection and credentials.
func (s *PmsService) ConnectPms(ctx context.Context, provider, label string, credentials map[string]string) (domain.Connection, error) {
	tc, err := platformauth.FromContext(ctx)
	if err != nil {
		return domain.Connection{}, err
	}
	if provider == "" {
		provider = "mypms"
	}
	ref := ""
	if len(credentials) > 0 && s.secrets != nil {
		ref, err = s.secrets.Store(ctx, "", credentials)
		if err != nil {
			return domain.Connection{}, fmt.Errorf("pms: store credentials: %w", err)
		}
	}
	conn := domain.Connection{
		ID:        uuid.NewString(),
		OrgID:     tc.OrgID,
		Provider:  provider,
		Name:      label,
		Status:    "active",
		SecretRef: ref,
	}
	return s.conns.Create(ctx, conn, credentials)
}

func (s *PmsService) ListConnections(ctx context.Context) ([]domain.Connection, error) {
	tc, err := platformauth.FromContext(ctx)
	if err != nil {
		return nil, err
	}
	return s.conns.ListByOrg(ctx, tc.OrgID)
}

func (s *PmsService) DisconnectPms(ctx context.Context, id string) (domain.Connection, error) {
	if err := s.conns.UpdateStatus(ctx, id, "disabled", ""); err != nil {
		return domain.Connection{}, err
	}
	return s.conns.GetByID(ctx, id)
}

func (s *PmsService) ListProperties(ctx context.Context, connectionID string) ([]domain.Property, error) {
	if connectionID != "" {
		return s.props.ListByConnection(ctx, connectionID)
	}
	tc, err := platformauth.FromContext(ctx)
	if err != nil {
		return nil, err
	}
	return s.props.ListByOrg(ctx, tc.OrgID)
}

func (s *PmsService) GetProperty(ctx context.Context, id string) (domain.Property, []domain.RoomType, error) {
	prop, err := s.props.GetByID(ctx, id)
	if err != nil {
		return domain.Property{}, nil, err
	}
	rts, err := s.ListRoomTypes(ctx, id)
	if err != nil {
		return prop, nil, err
	}
	return prop, rts, nil
}

func (s *PmsService) ListRoomTypes(ctx context.Context, propertyID string) ([]domain.RoomType, error) {
	rts, err := s.roomTypes.ListByProperty(ctx, propertyID)
	if err != nil {
		return nil, err
	}
	rooms, err := s.rooms.ListByProperty(ctx, propertyID)
	if err != nil {
		return nil, err
	}
	
	roomsByRT := make(map[string][]domain.Room)
	for _, r := range rooms {
		roomsByRT[r.RoomTypeID] = append(roomsByRT[r.RoomTypeID], r)
	}

	for i := range rts {
		rts[i].Rooms = roomsByRT[rts[i].ID]
	}

	return rts, nil
}

// SyncCatalog pulls properties and room types from the PMS into the local catalog.
func (s *PmsService) SyncCatalog(ctx context.Context, connectionID string, filter domain.PropertySearchFilter) (domain.SyncCatalogResult, error) {
	engine, conn, err := s.engineForConnection(ctx, connectionID)
	if err != nil {
		_ = s.conns.UpdateStatus(ctx, connectionID, "error", err.Error())
		return domain.SyncCatalogResult{}, err
	}

	remoteProps, err := engine.SearchProperties(ctx, filter)
	if err != nil {
		_ = s.conns.UpdateStatus(ctx, connectionID, "error", err.Error())
		return domain.SyncCatalogResult{}, fmt.Errorf("pms: search properties: %w", err)
	}

	tc, err := platformauth.FromContext(ctx)
	if err != nil {
		return domain.SyncCatalogResult{}, err
	}

	var propsSynced, rtsSynced int
	for _, rp := range remoteProps {
		if rp.ExternalID == "" {
			continue
		}
		saved, err := s.props.Upsert(ctx, domain.Property{
			OrgID:           tc.OrgID,
			ConnectionID:    conn.ID,
			ExternalID:      rp.ExternalID,
			Name:            rp.Name,
			Timezone:        rp.Timezone,
			DefaultCurrency: rp.DefaultCurrency,
			City:            rp.City,
			Country:         rp.Country,
			IsActive:        true,
		})
		if err != nil {
			s.log.Error("sync property failed", "external_id", rp.ExternalID, "err", err)
			continue
		}
		propsSynced++

		roomTypes, err := engine.ListRoomTypes(ctx, rp.ExternalID)
		if err != nil {
			s.log.Error("list room types failed", "property", rp.ExternalID, "err", err)
			continue
		}

		activeRTExtIDs := make(map[string]bool)
		activeRoomExtIDs := make(map[string]bool)

		for _, rt := range roomTypes {
			savedRT, err := s.roomTypes.Upsert(ctx, domain.RoomType{
				OrgID:        tc.OrgID,
				PropertyID:   saved.ID,
				ExternalID:   rt.ExternalID,
				Code:         rt.Code,
				Name:         rt.Name,
				MaxOccupancy: rt.MaxOccupancy,
				BaseOccupancy: rt.BaseOccupancy,
				IsActive:     true,
			})
			if err != nil {
				s.log.Error("upsert room type failed", "code", rt.Code, "err", err)
				continue
			}
			
			for _, rm := range rt.Rooms {
				s.log.Info("syncing room", "property", saved.ID, "room_type", savedRT.ID, "external_id", rm.ExternalID, "name", rm.Name)
				if rm.ExternalID != "" {
					activeRoomExtIDs[rm.ExternalID] = true
				}
				_, err := s.rooms.Upsert(ctx, domain.Room{
					OrgID:      tc.OrgID,
					PropertyID: saved.ID,
					RoomTypeID: savedRT.ID,
					ExternalID: rm.ExternalID,
					Name:       rm.Name,
					IsActive:   true,
				})
				if err != nil {
					s.log.Error("upsert room failed", "room", rm.Name, "err", err)
					return domain.SyncCatalogResult{}, fmt.Errorf("failed to sync room '%s': %w", rm.Name, err)
				}
			}

			if rt.ExternalID != "" {
				activeRTExtIDs[rt.ExternalID] = true
			} else if rt.Code != "" {
				activeRTExtIDs[rt.Code] = true
			}

			rtsSynced++
		}

		// Deactivate missing room types
		if existingRTs, err := s.roomTypes.ListByProperty(ctx, saved.ID); err == nil {
			for _, ert := range existingRTs {
				key := ert.ExternalID
				if key == "" {
					key = ert.Code
				}
				if key != "" && !activeRTExtIDs[key] && ert.IsActive {
					s.log.Info("deactivating missing room type", "property", saved.ID, "key", key, "name", ert.Name)
					ert.IsActive = false
					_, _ = s.roomTypes.Upsert(ctx, ert)
				}
			}
		}

		// Deactivate missing rooms
		if existingRooms, err := s.rooms.ListByProperty(ctx, saved.ID); err == nil {
			for _, er := range existingRooms {
				if er.ExternalID != "" && !activeRoomExtIDs[er.ExternalID] && er.IsActive {
					s.log.Info("deactivating missing room", "property", saved.ID, "external_id", er.ExternalID, "name", er.Name)
					er.IsActive = false
					_, _ = s.rooms.Upsert(ctx, er)
				}
			}
		}
	}

	_ = s.conns.UpdateLastSync(ctx, connectionID, time.Now().UTC())
	return domain.SyncCatalogResult{
		PropertiesSynced: propsSynced,
		RoomTypesSynced:  rtsSynced,
	}, nil
}

// IngestAvailability pulls availability from the PMS and writes inventory_days.
func (s *PmsService) IngestAvailability(ctx context.Context, propertyID string, q domain.AvailabilityQuery) (domain.IngestAvailabilityResult, error) {
	prop, err := s.props.GetByID(ctx, propertyID)
	if err != nil {
		return domain.IngestAvailabilityResult{}, fmt.Errorf("pms: get property: %w", err)
	}
	if prop.ConnectionID == "" || prop.ExternalID == "" {
		return domain.IngestAvailabilityResult{}, fmt.Errorf("pms: property missing connection or external id")
	}

	engine, _, err := s.engineForConnection(ctx, prop.ConnectionID)
	if err != nil {
		return domain.IngestAvailabilityResult{}, err
	}

	offers, err := engine.SearchAvailability(ctx, prop.ExternalID, q)
	if err != nil {
		return domain.IngestAvailabilityResult{}, fmt.Errorf("pms: search availability: %w", err)
	}

	tc, err := platformauth.FromContext(ctx)
	if err != nil {
		return domain.IngestAvailabilityResult{}, err
	}

	// Map external room type IDs to internal UUIDs.
	rtByExt := make(map[string]string)
	existing, _ := s.roomTypes.ListByProperty(ctx, propertyID)
	for _, rt := range existing {
		if rt.ExternalID != "" {
			rtByExt[rt.ExternalID] = rt.ID
		}
		rtByExt[rt.Code] = rt.ID
	}

	inputs := make([]ports.InventoryDayInput, 0)
	for d := q.Checkin; d.Before(q.Checkout); d = d.AddDate(0, 0, 1) {
		for _, o := range offers {
			rtID, ok := rtByExt[o.RoomTypeID]
			if !ok {
				s.log.Warn("unknown room type from PMS", "external_id", o.RoomTypeID)
				continue
			}
			avail := o.AvailableUnits
			stopSell := !o.IsAvailable
			inputs = append(inputs, ports.InventoryDayInput{
				RoomTypeID: rtID,
				StayDate:   d,
				Available:  avail,
				StopSell:   stopSell,
			})
		}
	}

	if s.inventory == nil {
		return domain.IngestAvailabilityResult{}, fmt.Errorf("pms: inventory writer not configured")
	}
	rows, eventID, err := s.inventory.BulkUpsertFromPMS(ctx, tc.OrgID, inputs)
	if err != nil {
		return domain.IngestAvailabilityResult{}, err
	}
	return domain.IngestAvailabilityResult{
		InventoryRowsAffected: rows,
		EventID:               eventID,
	}, nil
}

// OrgHealth proxies the PMS org health check.
func (s *PmsService) OrgHealth(ctx context.Context, connectionID string) (*domain.OrgHealth, error) {
	engine, _, err := s.engineForConnection(ctx, connectionID)
	if err != nil {
		return nil, err
	}
	return engine.OrgHealth(ctx)
}

// PropertyHealth proxies the PMS property health check.
func (s *PmsService) PropertyHealth(ctx context.Context, propertyID string) (*domain.PropertyHealth, error) {
	prop, err := s.props.GetByID(ctx, propertyID)
	if err != nil {
		return nil, err
	}
	engine, _, err := s.engineForConnection(ctx, prop.ConnectionID)
	if err != nil {
		return nil, err
	}
	return engine.PropertyHealth(ctx, prop.ExternalID)
}

// GetQuote proxies get_quote to the PMS.
func (s *PmsService) GetQuote(ctx context.Context, propertyID string, q domain.QuoteQuery) (*domain.Quote, error) {
	prop, err := s.props.GetByID(ctx, propertyID)
	if err != nil {
		return nil, err
	}
	engine, _, err := s.engineForConnection(ctx, prop.ConnectionID)
	if err != nil {
		return nil, err
	}
	return engine.GetQuote(ctx, prop.ExternalID, q)
}

// CreateBooking proxies create_booking to the PMS.
func (s *PmsService) CreateBooking(ctx context.Context, propertyID string, in domain.CreateBookingInput) (*domain.PmsBooking, error) {
	prop, err := s.props.GetByID(ctx, propertyID)
	if err != nil {
		return nil, err
	}
	engine, _, err := s.engineForConnection(ctx, prop.ConnectionID)
	if err != nil {
		return nil, err
	}
	return engine.CreateBooking(ctx, prop.ExternalID, in)
}

// GetBooking proxies get_booking to the PMS.
func (s *PmsService) GetBooking(ctx context.Context, propertyID, bookingID string) (*domain.PmsBooking, error) {
	prop, err := s.props.GetByID(ctx, propertyID)
	if err != nil {
		return nil, err
	}
	engine, _, err := s.engineForConnection(ctx, prop.ConnectionID)
	if err != nil {
		return nil, err
	}
	return engine.GetBooking(ctx, prop.ExternalID, bookingID)
}

// UpdateBooking proxies update_booking to the PMS.
func (s *PmsService) UpdateBooking(ctx context.Context, propertyID string, in domain.UpdateBookingInput) (*domain.PmsBooking, error) {
	prop, err := s.props.GetByID(ctx, propertyID)
	if err != nil {
		return nil, err
	}
	engine, _, err := s.engineForConnection(ctx, prop.ConnectionID)
	if err != nil {
		return nil, err
	}
	return engine.UpdateBooking(ctx, prop.ExternalID, in)
}

// CancelBooking proxies cancel_booking to the PMS.
func (s *PmsService) CancelBooking(ctx context.Context, propertyID, bookingID, reason string) (*domain.CancelBookingResult, error) {
	prop, err := s.props.GetByID(ctx, propertyID)
	if err != nil {
		return nil, err
	}
	engine, _, err := s.engineForConnection(ctx, prop.ConnectionID)
	if err != nil {
		return nil, err
	}
	return engine.CancelBooking(ctx, prop.ExternalID, bookingID, reason)
}

// DeleteBooking proxies delete_booking to the PMS.
func (s *PmsService) DeleteBooking(ctx context.Context, propertyID, bookingID string) (*domain.DeleteBookingResult, error) {
	prop, err := s.props.GetByID(ctx, propertyID)
	if err != nil {
		return nil, err
	}
	engine, _, err := s.engineForConnection(ctx, prop.ConnectionID)
	if err != nil {
		return nil, err
	}
	return engine.DeleteBooking(ctx, prop.ExternalID, bookingID)
}

// ListBookings proxies list_bookings to the PMS.
func (s *PmsService) ListBookings(ctx context.Context, propertyID string, in domain.ListBookingsInput) (*domain.ListBookingsResult, error) {
	prop, err := s.props.GetByID(ctx, propertyID)
	if err != nil {
		return nil, err
	}
	engine, _, err := s.engineForConnection(ctx, prop.ConnectionID)
	if err != nil {
		return nil, err
	}
	return engine.ListBookings(ctx, prop.ExternalID, in)
}
