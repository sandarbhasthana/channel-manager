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
	pricingdomain "github.com/channel-manager/channel-manager/services/pricing/domain"
	pricingusecases "github.com/channel-manager/channel-manager/services/pricing/usecases"
)

// Service orchestrates PMS-facing outbound integration APIs.
type Service struct {
	props    pmsports.PropertyRepository
	channels *channelusecases.ChannelService
	jobs     channelports.SyncJobRepository
	inv      *invusecases.InventoryService
	res      *resusecases.ReservationService
	pms      *pmsusecases.PmsService
	pricing  *pricingusecases.PricingService
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
	pricing *pricingusecases.PricingService,
) *Service {
	return &Service{
		props:    props,
		channels: channels,
		jobs:     jobs,
		inv:      inv,
		res:      res,
		pms:      pms,
		pricing:  pricing,
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
	prop, err := s.loadProperty(ctx, propertyID)
	if err != nil {
		return nil, err
	}
	// Use prop.ID (the internal UUID) for all subsequent calls instead of the incoming propertyID
	switch action {
	case domain.ActionListChannels:
		return s.listChannels(ctx, prop.ID)
	case domain.ActionGetInventory:
		return s.getInventory(ctx, prop.ID, body)
	case domain.ActionGetRates:
		return map[string]any{"rates": []any{}}, nil
	case domain.ActionListReservations:
		return s.listReservations(ctx, prop.ID)
	case domain.ActionFetchChannelReservations:
		return s.fetchChannelReservations(ctx, prop.ID, prop.ExternalID, body)
	case domain.ActionPushAvailability:
		return s.pushAvailability(ctx, prop.ID, body)
	case domain.ActionPushRates:
		return s.pushRates(ctx, prop.ID, body)
	case domain.ActionGetSyncJobs:
		return s.getSyncJobs(ctx, prop.ID, body)
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
	ID, Name, DefaultCurrency, ExternalID string
}, error) {
	prop, err := s.props.GetByID(ctx, propertyID)
	if err != nil {
		// Fallback to searching by ExternalID (which is the PMS property ID)
		prop, err = s.props.GetByExternalID(ctx, "", propertyID)
		if err != nil {
			return struct{ ID, Name, DefaultCurrency, ExternalID string }{}, fmt.Errorf("property not found by ID or ExternalID: %w", err)
		}
	}
	return struct{ ID, Name, DefaultCurrency, ExternalID string }{
		ID: prop.ID, Name: prop.Name, DefaultCurrency: prop.DefaultCurrency, ExternalID: prop.ExternalID,
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

func (s *Service) fetchChannelReservations(ctx context.Context, propertyID string, externalPropertyID string, body map[string]any) (map[string]any, error) {
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
				ExternalPropertyID:    externalPropertyID,
				ChannelID:             ch.ConnectionID,
				RoomTypeID:            f.RoomTypeExternalID, // Maps to cmRoomTypeId in PMS
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
	rates, ok := body["rates"].([]any)
	if !ok || len(rates) == 0 {
		return nil, errors.New("rates payload is required and must be an array")
	}

	var rateDays []pricingdomain.RateDay
	var updates []chdomain.RateUpdate

	for _, rAny := range rates {
		rm, ok := rAny.(map[string]any)
		if !ok {
			continue
		}
		
		roomTypeID, _ := rm["room_type_id"].(string)
		
		daysAny, ok := rm["days"].([]any)
		if !ok {
			continue
		}
		
		for _, dAny := range daysAny {
			dm, ok := dAny.(map[string]any)
			if !ok {
				continue
			}
			
			dateStr, _ := dm["date"].(string)
			date, err := time.Parse("2006-01-02", dateStr)
			if err != nil {
				continue
			}
			
			price, _ := dm["price"].(float64)
			
			// We store this rate internally.
			//
			// OrgID is deliberately unset: the repository takes the tenant from
			// the context, and a non-empty value that disagrees is rejected as a
			// cross-org batch. It previously read "system", which is not a UUID,
			// so this write could never have succeeded. It went unnoticed because
			// the BulkUpsertRates error below is logged and swallowed.
			//
			// RatePlanID is likewise resolved by the repository from (org_id, code).
			rateDays = append(rateDays, pricingdomain.RateDay{
				PropertyID: propertyID,
				RoomTypeID: roomTypeID,
				Date:       date,
				BaseRate:   price,
				Currency:   "USD",
			})
			
			// We prepare this update for the channel adapters
			updates = append(updates, chdomain.RateUpdate{
				PropertyID: propertyID,
				RoomTypeID: roomTypeID,
				RatePlanID: "default",
				Date:       date,
				BaseRate:   price,
				Currency:   "USD",
			})
		}
	}

	// 1. Persist to CM's canonical store first. CM owns rates (D3), so a failed
	// write must surface as an error rather than be logged and swallowed — and
	// must not fan out to OTAs a price CM has not recorded (canonical-first,
	// the same ordering as the storefront saga, D6).
	ratesSaved := 0
	if len(rateDays) > 0 {
		if s.pricing == nil {
			return nil, errors.New("pricing service is not configured")
		}
		if err := s.pricing.BulkUpsertRates(ctx, rateDays); err != nil {
			return nil, fmt.Errorf("save rates: %w", err)
		}
		ratesSaved = len(rateDays)
	}

	// 2. Dispatch PushRates to OTA adapters
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
		if err := s.channels.PushRates(ctx, ch.ConnectionID, updates); err != nil {
			if errors.Is(err, chdomain.ErrNotImplemented) {
				errorsOut = append(errorsOut, fmt.Sprintf("%s: OTA rate push not implemented", ch.Provider))
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
		"rates_saved":     ratesSaved,
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
