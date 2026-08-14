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
}

// SupportedProviders are the OTA adapters registered on the channel service.
// Exposed so the PMS "add connector" dialog does not have to hardcode a list
// that drifts from what the backend can actually dispatch to.
var SupportedProviders = []map[string]string{
	{"id": "airbnb", "name": "Airbnb"},
	{"id": "bookingcom", "name": "Booking.com"},
	{"id": "expedia", "name": "Expedia"},
	{"id": "agoda", "name": "Agoda"},
}
