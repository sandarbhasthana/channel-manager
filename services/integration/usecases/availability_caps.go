package usecases

import (
	"fmt"
	"time"

	chdomain "github.com/channel-manager/channel-manager/services/channel/domain"
)

// channelCaps maps channel id -> room type id -> the most rooms of that type
// the channel may sell on any night.
//
// The PMS lets a property sell fewer rooms on one OTA than on the others
// (a room type switched off there, or single rooms held back). Inventory is
// sold by type, so the PMS turns that into a ceiling per (channel, room type)
// and this service clamps each channel's copy of the availability to it.
// A channel or room type with no entry is not capped.
type channelCaps map[string]map[string]int

// parseChannelCaps reads `channel_caps: [{channel_id, room_type_id,
// max_available}]`. Malformed entries are skipped rather than failing the
// push: a bad cap must not stop availability reaching the other channels.
func parseChannelCaps(body map[string]any) channelCaps {
	out := channelCaps{}
	raw, ok := body["channel_caps"].([]any)
	if !ok {
		return out
	}
	for _, item := range raw {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		chID, _ := m["channel_id"].(string)
		rtID, _ := m["room_type_id"].(string)
		maxF, ok := m["max_available"].(float64)
		if chID == "" || rtID == "" || !ok {
			continue
		}
		maxN := int(maxF)
		if maxN < 0 {
			maxN = 0
		}
		if out[chID] == nil {
			out[chID] = map[string]int{}
		}
		out[chID][rtID] = maxN
	}
	return out
}

// applyCaps returns the updates one channel should receive. The input slice is
// never modified, because the same base updates are reused for every channel.
func applyCaps(updates []chdomain.AvailabilityUpdate, caps map[string]int) []chdomain.AvailabilityUpdate {
	if len(caps) == 0 {
		return updates
	}
	out := make([]chdomain.AvailabilityUpdate, len(updates))
	for i, u := range updates {
		if limit, ok := caps[u.RoomTypeID]; ok && u.Available > limit {
			u.Available = limit
		}
		if u.Available <= 0 {
			u.Available = 0
			u.StopSell = true
		}
		out[i] = u
	}
	return out
}

// parsePMSInventory reads the shape the PMS sends:
//
//	inventory: [{room_type_id, days: [{date, available, min_stay, max_stay, stop_sell}]}]
//
// In the bundled product the PMS owns availability (it holds the bookings and
// the room blocks), so its numbers are pushed as they are instead of being
// re-read from this service's inventory store. Before this path existed every
// PMS push was rejected for lacking checkin/checkout.
func parsePMSInventory(propertyID string, raw []any) ([]chdomain.AvailabilityUpdate, error) {
	var out []chdomain.AvailabilityUpdate
	for _, rtAny := range raw {
		rt, ok := rtAny.(map[string]any)
		if !ok {
			continue
		}
		roomTypeID, _ := rt["room_type_id"].(string)
		if roomTypeID == "" {
			return nil, fmt.Errorf("inventory entry is missing room_type_id")
		}
		days, _ := rt["days"].([]any)
		for _, dAny := range days {
			d, ok := dAny.(map[string]any)
			if !ok {
				continue
			}
			dateS, _ := d["date"].(string)
			date, err := time.Parse("2006-01-02", dateS)
			if err != nil {
				return nil, fmt.Errorf("invalid inventory date %q for room type %s", dateS, roomTypeID)
			}
			avail, _ := d["available"].(float64)
			minStay, _ := d["min_stay"].(float64)
			maxStay, _ := d["max_stay"].(float64)
			stopSell, _ := d["stop_sell"].(bool)
			out = append(out, chdomain.AvailabilityUpdate{
				PropertyID: propertyID,
				RoomTypeID: roomTypeID,
				Date:       date,
				Available:  int(avail),
				StopSell:   stopSell || avail <= 0,
				MinStay:    int(minStay),
				MaxStay:    int(maxStay),
			})
		}
	}
	return out, nil
}
