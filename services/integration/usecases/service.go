package usecases

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	chdomain "github.com/channel-manager/channel-manager/services/channel/domain"
	channelports "github.com/channel-manager/channel-manager/services/channel/ports"
	channelusecases "github.com/channel-manager/channel-manager/services/channel/usecases"
	"github.com/channel-manager/channel-manager/services/integration/domain"
	invdomain "github.com/channel-manager/channel-manager/services/inventory/domain"
	invusecases "github.com/channel-manager/channel-manager/services/inventory/usecases"
	pmsports "github.com/channel-manager/channel-manager/services/pms/ports"
	pmsusecases "github.com/channel-manager/channel-manager/services/pms/usecases"
	pmsdomain "github.com/channel-manager/channel-manager/services/pms/domain"
	resdomain "github.com/channel-manager/channel-manager/services/reservations/domain"
	resusecases "github.com/channel-manager/channel-manager/services/reservations/usecases"
)

// Service orchestrates PMS-facing outbound integration APIs.
type Service struct {
	props    pmsports.PropertyRepository
	channels *channelusecases.ChannelService
	jobs     channelports.SyncJobRepository
	inv      *invusecases.InventoryService
	res      *resusecases.ReservationService
	pms      *pmsusecases.PmsService
	log      *slog.Logger
}

// NewService creates an integration service.
func NewService(
	props pmsports.PropertyRepository,
	channels *channelusecases.ChannelService,
	jobs channelports.SyncJobRepository,
	inv *invusecases.InventoryService,
	res *resusecases.ReservationService,
	pms *pmsusecases.PmsService,
) *Service {
	return &Service{
		props:    props,
		channels: channels,
		jobs:     jobs,
		inv:      inv,
		res:      res,
		pms:      pms,
		log:      slog.Default().With("service", "integration"),
	}
}

// OrgHealth returns org-level integration health.
func (s *Service) OrgHealth(ctx context.Context, orgID string) map[string]any {
	return map[string]any{
		"status":             "ok",
		"service":            "channel-manager-integration",
		"organization_id":    orgID,
		"available_actions":  domain.OrgAvailableActions,
	}
}

// PropertyHealth returns property-level health with channel summary.
func (s *Service) PropertyHealth(ctx context.Context, propertyID string) (map[string]any, error) {
	prop, err := s.loadProperty(ctx, propertyID)
	if err != nil {
		return nil, err
	}
	channels, err := s.channels.ListChannels(ctx, propertyID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"status":  "ok",
		"service": "channel-manager-integration",
		"property": map[string]any{
			"property_id": prop.ID,
			"name":        prop.Name,
			"currency":    prop.DefaultCurrency,
		},
		"channels_count":     len(channels),
		"available_actions":  domain.OrgAvailableActions,
	}, nil
}

// Dispatch runs a property-scoped action from the POST body.
func (s *Service) Dispatch(ctx context.Context, propertyID, action string, body map[string]any) (any, error) {
	if _, err := s.loadProperty(ctx, propertyID); err != nil {
		return nil, err
	}
	switch action {
	case domain.ActionListChannels:
		return s.listChannels(ctx, propertyID)
	case domain.ActionGetInventory:
		return s.getInventory(ctx, propertyID, body)
	case domain.ActionGetRates:
		return map[string]any{"rates": []any{}}, nil
	case domain.ActionListReservations:
		return s.listReservations(ctx, propertyID)
	case domain.ActionFetchChannelReservations:
		return s.fetchChannelReservations(ctx, propertyID, body)
	case domain.ActionPushAvailability:
		return s.pushAvailability(ctx, propertyID, body)
	case domain.ActionPushRates:
		return s.pushRates(ctx, propertyID, body)
	case domain.ActionGetSyncJobs:
		return s.getSyncJobs(ctx, propertyID, body)
	default:
		return nil, fmt.Errorf("unknown action %q", action)
	}
}

// OrgDispatch runs an organization-scoped action from the POST body.
func (s *Service) OrgDispatch(ctx context.Context, action string, body map[string]any) (any, error) {
	switch action {
	case "sync_catalog":
		conns, err := s.pms.ListConnections(ctx)
		if err != nil {
			return nil, err
		}
		var connID string
		if len(conns) == 0 {
			baseURL, _ := body["base_url"].(string)
			if baseURL == "" {
				return nil, errors.New("no PMS connections found for organization, and no base_url provided for auto-registration")
			}
			token, _ := body["token"].(string)
			if token == "" {
				token = "auto-registered-dummy-token" // Fallback to prevent credential errors
			}
			s.log.Info("Auto-registering PMS", "base_url", baseURL, "token", token)
			creds := map[string]string{
				"base_url":     baseURL,
				"bearer_token": token,
				"token":        token,
			}
			newConn, err := s.pms.ConnectPms(ctx, "mypms", "Auto-Registered PMS", creds)
			if err != nil {
				return nil, fmt.Errorf("failed to auto-register PMS: %w", err)
			}
			connID = newConn.ID
		} else {
			connID = conns[0].ID
			if id, ok := body["connection_id"].(string); ok && id != "" {
				found := false
				for _, c := range conns {
					if c.ID == id {
						found = true
						break
					}
				}
				if !found {
					return nil, errors.New("invalid connection_id")
				}
				connID = id
			}
		}
		res, err := s.pms.SyncCatalog(ctx, connID, pmsdomain.PropertySearchFilter{})
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"properties_synced": res.PropertiesSynced,
			"room_types_synced": res.RoomTypesSynced,
		}, nil
	default:
		return nil, fmt.Errorf("unknown action %q", action)
	}
}

func (s *Service) loadProperty(ctx context.Context, propertyID string) (struct {
	ID, Name, DefaultCurrency string
}, error) {
	prop, err := s.props.GetByID(ctx, propertyID)
	if err != nil {
		return struct{ ID, Name, DefaultCurrency string }{}, fmt.Errorf("property not found: %w", err)
	}
	return struct{ ID, Name, DefaultCurrency string }{
		ID: prop.ID, Name: prop.Name, DefaultCurrency: prop.DefaultCurrency,
	}, nil
}

func (s *Service) listChannels(ctx context.Context, propertyID string) (map[string]any, error) {
	channels, err := s.channels.ListChannels(ctx, propertyID)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(channels))
	for _, ch := range channels {
		out = append(out, map[string]any{
			"id":                   ch.ID,
			"connection_id":        ch.ConnectionID,
			"provider":             ch.Provider,
			"external_property_id": ch.ExternalPropertyID,
			"status":               ch.Status,
			"last_sync_at":         ch.LastSyncAt,
			"last_error":           ch.LastError,
		})
	}
	return map[string]any{"channels": out}, nil
}

func (s *Service) getInventory(ctx context.Context, propertyID string, body map[string]any) (map[string]any, error) {
	checkin, checkout, err := parseDateRange(body)
	if err != nil {
		return nil, err
	}
	roomTypeID, _ := body["room_type_id"].(string)
	if roomTypeID == "" {
		return nil, errors.New("room_type_id is required")
	}
	days, err := s.inv.GetInventory(ctx, invusecases.GetInventoryInput{
		RoomTypeID: roomTypeID,
		From:       checkin,
		To:         checkout.AddDate(0, 0, -1),
	})
	if err != nil {
		return nil, err
	}
	rows := make([]map[string]any, 0, len(days))
	for _, d := range days {
		rows = append(rows, map[string]any{
			"room_type_id": d.RoomTypeID,
			"stay_date":    d.StayDate.Format("2006-01-02"),
			"available":    d.Available,
			"stop_sell":    d.StopSell,
			"min_stay":     d.MinStay,
			"max_stay":     d.MaxStay,
			"cta":          d.CTA,
			"ctd":          d.CTD,
		})
	}
	return map[string]any{"days": rows}, nil
}

func (s *Service) listReservations(ctx context.Context, propertyID string) (map[string]any, error) {
	list, err := s.res.ListReservations(ctx, propertyID)
	if err != nil {
		return nil, err
	}
	rows := make([]map[string]any, 0, len(list))
	for _, r := range list {
		rows = append(rows, reservationJSON(r))
	}
	return map[string]any{"reservations": rows}, nil
}

func (s *Service) fetchChannelReservations(ctx context.Context, propertyID string, body map[string]any) (map[string]any, error) {
	since := time.Now().AddDate(0, 0, -30)
	if v, ok := body["since"].(string); ok && v != "" {
		t, err := time.Parse("2006-01-02", v)
		if err != nil {
			return nil, fmt.Errorf("invalid since date")
		}
		since = t
	}
	idem, _ := body["idempotency_key"].(string)

	channels, err := s.channels.ListChannels(ctx, propertyID)
	if err != nil {
		return nil, err
	}
	var ingested int
	var fetchErrors []string
	for _, ch := range channels {
		if ch.Status != "active" {
			continue
		}
		fetched, err := s.channels.FetchReservations(ctx, ch.ConnectionID, ch.ExternalPropertyID, since)
		if err != nil {
			if errors.Is(err, chdomain.ErrNotImplemented) {
				fetchErrors = append(fetchErrors, fmt.Sprintf("%s: OTA adapter not implemented", ch.Provider))
				continue
			}
			fetchErrors = append(fetchErrors, fmt.Sprintf("%s: %v", ch.Provider, err))
			continue
		}
		for _, f := range fetched {
			res := &resdomain.Reservation{
				PropertyID:            propertyID,
				ChannelID:             ch.ConnectionID,
				GuestName:             f.GuestName,
				CheckIn:               f.CheckIn,
				CheckOut:              f.CheckOut,
				Status:                f.Status,
				TotalAmount:           f.TotalAmount,
				Currency:              f.Currency,
				ChannelConfirmationID: f.ChannelConfirmationID,
			}
			if res.Status == "" {
				res.Status = "confirmed"
			}
			if _, _, err := s.res.IngestReservation(ctx, res, idem+":"+f.ChannelConfirmationID); err != nil {
				s.log.Warn("ingest reservation failed", "err", err)
				continue
			}
			ingested++
		}
	}
	return map[string]any{
		"ingested":     ingested,
		"fetch_errors": fetchErrors,
	}, nil
}

func (s *Service) pushAvailability(ctx context.Context, propertyID string, body map[string]any) (map[string]any, error) {
	checkin, checkout, err := parseDateRange(body)
	if err != nil {
		return nil, err
	}
	roomTypeID, _ := body["room_type_id"].(string)
	if roomTypeID == "" {
		return nil, errors.New("room_type_id is required")
	}
	days, err := s.inv.GetInventory(ctx, invusecases.GetInventoryInput{
		RoomTypeID: roomTypeID,
		From:       checkin,
		To:         checkout.AddDate(0, 0, -1),
	})
	if err != nil {
		return nil, err
	}
	updates := inventoryToChannelUpdates(propertyID, roomTypeID, days)
	channels, err := s.channels.ListChannels(ctx, propertyID)
	if err != nil {
		return nil, err
	}
	var pushed, failed int
	var errorsOut []string
	for _, ch := range channels {
		if ch.Status != "active" {
			continue
		}
		if err := s.channels.PushAvailability(ctx, ch.ConnectionID, updates); err != nil {
			if errors.Is(err, chdomain.ErrNotImplemented) {
				errorsOut = append(errorsOut, fmt.Sprintf("%s: OTA push not implemented", ch.Provider))
			} else {
				errorsOut = append(errorsOut, fmt.Sprintf("%s: %v", ch.Provider, err))
			}
			failed++
			continue
		}
		pushed++
	}
	return map[string]any{
		"channels_pushed": pushed,
		"channels_failed": failed,
		"errors":          errorsOut,
	}, nil
}

func (s *Service) pushRates(ctx context.Context, propertyID string, body map[string]any) (map[string]any, error) {
	_ = propertyID
	_ = body
	return map[string]any{
		"status":  "skipped",
		"message": "pricing service not yet wired; push_rates is a no-op",
	}, nil
}

func (s *Service) getSyncJobs(ctx context.Context, propertyID string, body map[string]any) (map[string]any, error) {
	limit := int32(20)
	if v, ok := body["limit"].(float64); ok && v > 0 {
		limit = int32(v)
	}
	channels, err := s.channels.ListChannels(ctx, propertyID)
	if err != nil {
		return nil, err
	}
	var jobs []map[string]any
	for _, ch := range channels {
		rows, err := s.jobs.ListRecentByConnection(ctx, ch.ConnectionID, limit)
		if err != nil {
			continue
		}
		for _, j := range rows {
			jobs = append(jobs, map[string]any{
				"id":              j.ID,
				"connection_id":   j.ConnectionID,
				"provider":        ch.Provider,
				"job_type":        j.JobType,
				"status":          j.Status,
				"last_error":      j.LastError,
				"scheduled_at":    j.ScheduledAt,
				"finished_at":     j.FinishedAt,
			})
		}
	}
	return map[string]any{"sync_jobs": jobs}, nil
}

func inventoryToChannelUpdates(propertyID, roomTypeID string, days []invdomain.InventoryDay) []chdomain.AvailabilityUpdate {
	out := make([]chdomain.AvailabilityUpdate, 0, len(days))
	for _, d := range days {
		minStay, maxStay := 0, 0
		if d.MinStay != nil {
			minStay = int(*d.MinStay)
		}
		if d.MaxStay != nil {
			maxStay = int(*d.MaxStay)
		}
		out = append(out, chdomain.AvailabilityUpdate{
			PropertyID: propertyID,
			RoomTypeID: roomTypeID,
			Date:       d.StayDate,
			Available:  d.Available,
			StopSell:   d.StopSell,
			MinStay:    minStay,
			MaxStay:    maxStay,
		})
	}
	return out
}

func reservationJSON(r resdomain.Reservation) map[string]any {
	return map[string]any{
		"id":                        r.ID,
		"property_id":               r.PropertyID,
		"channel_connection_id":     r.ChannelID,
		"guest_name":                r.GuestName,
		"check_in":                  r.CheckIn.Format("2006-01-02"),
		"check_out":                 r.CheckOut.Format("2006-01-02"),
		"status":                    r.Status,
		"total_amount":              r.TotalAmount,
		"currency":                  r.Currency,
		"channel_confirmation_id":   r.ChannelConfirmationID,
	}
}

func parseDateRange(body map[string]any) (time.Time, time.Time, error) {
	checkinS, _ := body["checkin"].(string)
	checkoutS, _ := body["checkout"].(string)
	if checkinS == "" || checkoutS == "" {
		return time.Time{}, time.Time{}, errors.New("checkin and checkout are required (YYYY-MM-DD)")
	}
	checkin, err := time.Parse("2006-01-02", checkinS)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid checkin")
	}
	checkout, err := time.Parse("2006-01-02", checkoutS)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid checkout")
	}
	return checkin, checkout, nil
}
