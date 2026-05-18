package domain

// Action names for POST /api/integrations/pms/{propertyId}.
const (
	ActionListChannels              = "list_channels"
	ActionGetInventory              = "get_inventory"
	ActionGetRates                  = "get_rates"
	ActionListReservations          = "list_reservations"
	ActionFetchChannelReservations  = "fetch_channel_reservations"
	ActionPushAvailability          = "push_availability"
	ActionPushRates                 = "push_rates"
	ActionGetSyncJobs               = "get_sync_jobs"
)

// Org-level available actions returned by health endpoints.
var OrgAvailableActions = []string{
	ActionListChannels,
	ActionGetInventory,
	ActionGetRates,
	ActionListReservations,
	ActionFetchChannelReservations,
	ActionPushAvailability,
	ActionPushRates,
	ActionGetSyncJobs,
}
