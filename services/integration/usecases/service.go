package usecases

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	platformauth "github.com/channel-manager/channel-manager/platform/auth"
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

// defaultPropertySetter is the slice of the property repository needed to elect a
// default. Declared here, and obtained by type assertion, so the shared
// PropertyRepository port does not have to grow a method that only provisioning
// uses.
type defaultPropertySetter interface {
	SetDefault(ctx context.Context, id string) error
	ListListings(ctx context.Context) ([]pmsdomain.PropertyListing, error)
}

// ensureDefaultProperty elects a default property for the org if none is set.
//
// Bundled tenants are created on first contact and have no dashboard to visit,
// so nothing would ever elect one — and `PUT /admin/default-property`, the only
// writer, sits behind a WorkOS session they do not have. The booking engine reads
// this value, so without it a freshly provisioned tenant has properties it cannot
// sell.
//
// Best-effort by design: a failure here must not fail the caller's actual
// request. Distribution works without a default; only the booking engine cares.
func (s *Service) ensureDefaultProperty(ctx context.Context) {
	repo, ok := s.props.(defaultPropertySetter)
	if !ok {
		return
	}
	listings, err := repo.ListListings(ctx)
	if err != nil || len(listings) == 0 {
		return
	}
	for _, l := range listings {
		if l.IsDefault {
			return
		}
	}
	// ListListings is already ordered and org-scoped by RLS, so the first entry is
	// a stable choice — the same property every time this runs, rather than one
	// that changes with row ordering.
	if err := repo.SetDefault(ctx, listings[0].ID); err != nil {
		s.log.Warn("could not elect a default property", "property_id", listings[0].ID, "err", err)
		return
	}
	s.log.Info("elected default property for tenant", "property_id", listings[0].ID)
}

// Dispatch runs a property-scoped action from the POST body.
func (s *Service) Dispatch(ctx context.Context, propertyID, action string, body map[string]any) (any, error) {
	prop, err := s.loadProperty(ctx, propertyID)
	if err != nil {
		return nil, err
	}
	// Deliberately NOT calling ensureDefaultProperty here. Electing a default
	// costs a tenant-scoped transaction and a property scan, and this runs on
	// every property-scoped call — overwhelmingly often to discover that a default
	// already exists. It belongs on the provisioning path (sync_catalog), which is
	// the one call every tenant makes before it can do anything else.
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
	case domain.ActionListConnections:
		return s.listConnections(ctx, prop.ID)
	case domain.ActionCreateConnection:
		return s.createConnection(ctx, prop.ID, prop.ExternalID, body)
	case domain.ActionUpdateConnection:
		return s.updateConnection(ctx, body)
	case domain.ActionDeleteConnection:
		return s.deleteConnection(ctx, body)
	case domain.ActionConnectChannel:
		return s.connectChannel(ctx, prop.ID, prop.ExternalID, body)
	case domain.ActionPauseChannel:
		return s.setChannelState(ctx, prop.ID, body, "pause")
	case domain.ActionResumeChannel:
		return s.setChannelState(ctx, prop.ID, body, "resume")
	case domain.ActionDisconnectChannel:
		return s.setChannelState(ctx, prop.ID, body, "disconnect")
	case domain.ActionListRoomTypes:
		return s.listRoomTypes(ctx, prop.ID)
	default:
		return nil, fmt.Errorf("unknown action %q", action)
	}
}

// OrgDispatch runs an organization-scoped action from the POST body.
func (s *Service) OrgDispatch(ctx context.Context, action string, body map[string]any) (any, error) {
	switch action {
	case "sync_catalog":
		defer s.ensureDefaultProperty(ctx)
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
				// Previously this defaulted to "auto-registered-dummy-token" so
				// registration would not error. It succeeded and then every
				// callback into that PMS was rejected — a connection that looks
				// healthy in every listing and works for nothing. Failing here
				// costs one clear error instead.
				return nil, errors.New("token is required when registering a PMS base_url")
			}
			// Note the token is NOT logged: it is the credential this service will
			// present when calling back into that PMS.
			s.log.Info("Auto-registering PMS", "base_url", baseURL)
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

// listConnections returns the org's OTA connections joined with this property's
// channel rows.
//
// The two are separate concepts in the domain — a Connection is an org-level
// credential, a Channel is one property listed against it — but a property
// operator thinks in terms of one thing ("is Booking.com on for my hotel?").
// Joining here rather than in the PMS keeps that decision in one place and
// spares the UI a second round trip just to correlate ids.
func (s *Service) listConnections(ctx context.Context, propertyID string) (map[string]any, error) {
	tc, err := platformauth.FromContext(ctx)
	if err != nil {
		return nil, err
	}
	conns, err := s.channels.ListConnections(ctx, tc.OrgID)
	if err != nil {
		return nil, err
	}
	channels, err := s.channels.ListChannels(ctx, propertyID)
	if err != nil {
		return nil, err
	}
	byConn := make(map[string]chdomain.Channel, len(channels))
	for _, ch := range channels {
		byConn[ch.ConnectionID] = ch
	}

	out := make([]map[string]any, 0, len(conns))
	for _, c := range conns {
		row := map[string]any{
			"id":           c.ID,
			"provider":     c.Provider,
			"name":         c.Name,
			"status":       c.Status,
			"last_sync_at": c.LastSyncAt,
			"last_error":   c.LastError,
			"created_at":   c.CreatedAt,
			// has_credentials rather than the secret itself: the PMS only needs
			// to know whether the connector is configured, and forwarding the
			// ref would leak a secret handle into a browser for no gain.
			"has_credentials": c.SecretRef != "",
			"linked":          false,
		}
		if ch, ok := byConn[c.ID]; ok {
			row["linked"] = true
			row["channel_id"] = ch.ID
			row["channel_status"] = ch.Status
			row["external_property_id"] = ch.ExternalPropertyID
			row["channel_last_sync_at"] = ch.LastSyncAt
			row["channel_last_error"] = ch.LastError
		}
		out = append(out, row)
	}
	return map[string]any{
		"connections": out,
		"providers":   domain.SupportedProviders,
	}, nil
}

// createConnection creates an org-level connection and, unless the caller opts
// out, immediately links the calling property to it.
//
// The link is the default because this action is only reachable from a
// property-scoped endpoint: a PMS user who adds Booking.com from inside their
// property is asking for their property to be on Booking.com, not for a
// dangling org credential they then have to attach in a second step.
func (s *Service) createConnection(ctx context.Context, propertyID, externalPropertyID string, body map[string]any) (map[string]any, error) {
	tc, err := platformauth.FromContext(ctx)
	if err != nil {
		return nil, err
	}
	provider, _ := body["provider"].(string)
	if provider == "" {
		return nil, errors.New("provider is required")
	}
	name, _ := body["name"].(string)
	if name == "" {
		name = provider
	}

	creds := stringMap(body["credentials"])

	conn, err := s.channels.CreateConnection(ctx, chdomain.Connection{
		OrgID:    tc.OrgID,
		Provider: provider,
		Name:     name,
		Status:   "active",
	}, creds)
	if err != nil {
		return nil, err
	}

	result := map[string]any{"connection_id": conn.ID, "provider": conn.Provider, "status": conn.Status}

	if link, ok := body["link_property"].(bool); ok && !link {
		return result, nil
	}
	extID, _ := body["external_property_id"].(string)
	if extID == "" {
		extID = externalPropertyID
	}
	ch, err := s.channels.ConnectChannel(ctx, chdomain.Channel{
		OrgID:              tc.OrgID,
		PropertyID:         propertyID,
		ConnectionID:       conn.ID,
		ExternalPropertyID: extID,
		Status:             "active",
	})
	if err != nil {
		// The credential was created and is usable; only the property link
		// failed. Report that rather than unwinding, so a retry of
		// connect_channel finishes the job instead of re-entering the API key.
		s.log.Warn("connection created but property link failed", "connection_id", conn.ID, "err", err)
		result["link_error"] = err.Error()
		return result, nil
	}
	result["channel_id"] = ch.ID
	result["linked"] = true
	return result, nil
}

func (s *Service) updateConnection(ctx context.Context, body map[string]any) (map[string]any, error) {
	id, _ := body["connection_id"].(string)
	if id == "" {
		return nil, errors.New("connection_id is required")
	}
	// Guard against updating a connection belonging to another org. RLS already
	// scopes the read, so a miss here means "not yours" as much as "not there".
	if _, err := s.channels.GetConnection(ctx, id); err != nil {
		return nil, fmt.Errorf("connection not found: %w", err)
	}
	name, _ := body["name"].(string)
	status, _ := body["status"].(string)
	if status != "" && status != "active" && status != "inactive" && status != "disabled" {
		return nil, fmt.Errorf("invalid status %q", status)
	}
	if err := s.channels.UpdateConnection(ctx, id, name, stringMap(body["credentials"]), status); err != nil {
		return nil, err
	}
	return map[string]any{"connection_id": id, "updated": true}, nil
}

func (s *Service) deleteConnection(ctx context.Context, body map[string]any) (map[string]any, error) {
	id, _ := body["connection_id"].(string)
	if id == "" {
		return nil, errors.New("connection_id is required")
	}
	if _, err := s.channels.GetConnection(ctx, id); err != nil {
		return nil, fmt.Errorf("connection not found: %w", err)
	}
	if err := s.channels.DeleteConnection(ctx, id); err != nil {
		return nil, err
	}
	return map[string]any{"connection_id": id, "deleted": true}, nil
}

// connectChannel links this property to an existing org connection.
func (s *Service) connectChannel(ctx context.Context, propertyID, externalPropertyID string, body map[string]any) (map[string]any, error) {
	tc, err := platformauth.FromContext(ctx)
	if err != nil {
		return nil, err
	}
	connID, _ := body["connection_id"].(string)
	if connID == "" {
		return nil, errors.New("connection_id is required")
	}
	// Linking the same connection twice would produce two channel rows for one
	// property, and every push loop below iterates channels — so the property
	// would get double pushes with no way to tell from the UI.
	existing, err := s.channels.ListChannels(ctx, propertyID)
	if err != nil {
		return nil, err
	}
	for _, ch := range existing {
		if ch.ConnectionID == connID {
			return map[string]any{"channel_id": ch.ID, "already_linked": true}, nil
		}
	}
	extID, _ := body["external_property_id"].(string)
	if extID == "" {
		extID = externalPropertyID
	}
	ch, err := s.channels.ConnectChannel(ctx, chdomain.Channel{
		OrgID:              tc.OrgID,
		PropertyID:         propertyID,
		ConnectionID:       connID,
		ExternalPropertyID: extID,
		Status:             "active",
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"channel_id": ch.ID, "status": ch.Status, "linked": true}, nil
}

// setChannelState pauses, resumes or disconnects a property's channel. The three
// share a body shape and an ownership check, so they share an implementation.
func (s *Service) setChannelState(ctx context.Context, propertyID string, body map[string]any, op string) (map[string]any, error) {
	id, _ := body["channel_id"].(string)
	if id == "" {
		return nil, errors.New("channel_id is required")
	}

	// The channel must belong to the property in the URL. RLS bounds this to the
	// caller's org, which is not enough on its own: a key scoped to one property
	// could otherwise pause a sibling property's channel just by passing its id,
	// silently halting that property's distribution.
	existing, err := s.channels.GetChannel(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("channel not found: %w", err)
	}
	if existing.PropertyID != propertyID {
		return nil, errors.New("channel does not belong to this property")
	}

	var ch chdomain.Channel
	switch op {
	case "pause":
		ch, err = s.channels.PauseChannel(ctx, id)
	case "resume":
		ch, err = s.channels.ResumeChannel(ctx, id)
	case "disconnect":
		ch, err = s.channels.DisconnectChannel(ctx, id)
	default:
		return nil, fmt.Errorf("unknown channel operation %q", op)
	}
	if err != nil {
		return nil, err
	}
	return map[string]any{"channel_id": ch.ID, "status": ch.Status}, nil
}

// listRoomTypes returns CM's catalog for the property, so the PMS mapping screen
// can present a picker. Free-text CM room type ids were the single biggest
// source of silent mapping failures: a typo produces a mapping that saves fine
// and then never matches anything on push.
func (s *Service) listRoomTypes(ctx context.Context, propertyID string) (map[string]any, error) {
	rts, err := s.pms.ListRoomTypes(ctx, propertyID)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(rts))
	for _, rt := range rts {
		out = append(out, map[string]any{
			"id":            rt.ID,
			"external_id":   rt.ExternalID,
			"code":          rt.Code,
			"name":          rt.Name,
			"max_occupancy": rt.MaxOccupancy,
			"rooms_count":   len(rt.Rooms),
			"is_active":     rt.IsActive,
		})
	}
	return map[string]any{"room_types": out}, nil
}

// stringMap coerces a decoded JSON object into map[string]string, dropping
// non-string values rather than failing: credential blobs are provider-shaped
// and a stray number should not sink the whole request.
func stringMap(v any) map[string]string {
	m, ok := v.(map[string]any)
	if !ok || len(m) == 0 {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, val := range m {
		if s, ok := val.(string); ok && s != "" {
			out[k] = s
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
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
