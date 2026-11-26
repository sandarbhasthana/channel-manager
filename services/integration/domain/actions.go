package domain

// Action names for POST /api/integrations/pms/{propertyId}.
//
// This is the API-key-authenticated surface the PMS talks to. It exists in
// parallel to the Connect-RPC services, which are gated behind a WorkOS user
// session — a PMS operator has no WorkOS identity, so anything the PMS UI needs
// to show or change has to be reachable from here or it is not reachable at all.
const (
	ActionListChannels             = "list_channels"
	ActionGetInventory             = "get_inventory"
	ActionGetRates                 = "get_rates"
	ActionListReservations         = "list_reservations"
	ActionFetchChannelReservations = "fetch_channel_reservations"
	ActionPushAvailability         = "push_availability"
	ActionPushRates                = "push_rates"
	ActionGetSyncJobs              = "get_sync_jobs"
	// ActionRedeliverReservations resends OTA bookings the PMS hasn't taken
	// yet (it was unreachable when they were first announced).
	ActionRedeliverReservations = "redeliver_reservations"

	// Connector management, mirroring the dashboard's /dashboard/connectors
	// screen so the PMS can own the whole OTA lifecycle without a second login.
	ActionListConnections   = "list_connections"
	ActionCreateConnection  = "create_connection"
	ActionUpdateConnection  = "update_connection"
	ActionDeleteConnection  = "delete_connection"
	ActionConnectChannel    = "connect_channel"
	ActionPauseChannel      = "pause_channel"
	ActionResumeChannel     = "resume_channel"
	ActionDisconnectChannel = "disconnect_channel"

	// Catalog reads, so the PMS mapping UI can offer a picker of the room types
	// CM actually knows about instead of a free-text id field.
	ActionListRoomTypes = "list_room_types"

	// Dashboard features for bundled (PMS-only) tenants, who have no WorkOS
	// login: same stores the standalone dashboard's /admin routes and Connect
	// services use. See usecases/dashboard_actions.go.
	ActionGetChannelRateRules         = "get_channel_rate_rules"
	ActionSaveChannelRateRules        = "save_channel_rate_rules"
	ActionGetBookingEngineSettings    = "get_booking_engine_settings"
	ActionUpdateBookingEngineSettings = "update_booking_engine_settings"
	ActionListPromoCodes              = "list_promo_codes"
	ActionCreatePromoCode             = "create_promo_code"
	ActionUpdatePromoCode             = "update_promo_code"
	ActionDeletePromoCode             = "delete_promo_code"
	ActionGetDefaultProperty          = "get_default_property"
	ActionSetDefaultProperty          = "set_default_property"
)

// OrgAvailableActions is advertised by both health endpoints. The PMS reads it
// to decide which screens to render, so an older CM deployment degrades to
// hiding the new tabs rather than showing buttons that 400.
var OrgAvailableActions = []string{
	ActionListChannels,
	ActionGetInventory,
	ActionGetRates,
	ActionListReservations,
	ActionFetchChannelReservations,
	ActionRedeliverReservations,
	ActionPushAvailability,
	ActionPushRates,
	ActionGetSyncJobs,
	ActionListConnections,
	ActionCreateConnection,
	ActionUpdateConnection,
	ActionDeleteConnection,
	ActionConnectChannel,
	ActionPauseChannel,
	ActionResumeChannel,
	ActionDisconnectChannel,
	ActionListRoomTypes,
	ActionGetChannelRateRules,
	ActionSaveChannelRateRules,
	ActionGetBookingEngineSettings,
	ActionUpdateBookingEngineSettings,
	ActionListPromoCodes,
	ActionCreatePromoCode,
	ActionUpdatePromoCode,
	ActionDeletePromoCode,
	ActionGetDefaultProperty,
	ActionSetDefaultProperty,
}

// Provider is an OTA the channel service has an adapter for. Live is false
// for adapters that are placeholders: they accept a connector but send
// nothing, so the PMS must not show them as selling or synced.
type Provider struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Live bool   `json:"live"`
}

// SupportedProviders are the OTA adapters registered on the channel service.
// Exposed so the PMS "add connector" dialog does not have to hardcode a list
// that drifts from what the backend can actually dispatch to.
var SupportedProviders = []Provider{
	{ID: "airbnb", Name: "Airbnb", Live: false},
	{ID: "bookingcom", Name: "Booking.com", Live: true},
	{ID: "expedia", Name: "Expedia", Live: true},
	{ID: "agoda", Name: "Agoda", Live: false},
}

// ProviderName is the display name of a provider id ("bookingcom" →
// "Booking.com"). Unknown ids come back unchanged.
func ProviderName(id string) string {
	for _, p := range SupportedProviders {
		if p.ID == id {
			return p.Name
		}
	}
	return id
}

// ProviderLive reports whether the provider's adapter really talks to the OTA.
func ProviderLive(id string) bool {
	for _, p := range SupportedProviders {
		if p.ID == id {
			return p.Live
		}
	}
	return false
}
