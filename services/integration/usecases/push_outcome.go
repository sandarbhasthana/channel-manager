package usecases

import (
	"errors"
	"time"

	chdomain "github.com/channel-manager/channel-manager/services/channel/domain"
	"github.com/channel-manager/channel-manager/services/integration/domain"
)

// pushOutcome collects what one availability or rate push did on each of the
// property's channels. The response answers 200 even when a channel failed,
// so the per-channel results are the only honest account of the push; the
// PMS reads them to say "Booking.com didn't get the update" instead of a
// blanket success.
type pushOutcome struct {
	pushed, failed, skipped int
	errors                  []string
	results                 []map[string]any
}

func (o *pushOutcome) record(ch chdomain.Channel, err error) {
	r := map[string]any{
		"channel_id":    ch.ID,
		"connection_id": ch.ConnectionID,
		"provider":      ch.Provider,
		"name":          domain.ProviderName(ch.Provider),
	}
	switch {
	case err == nil:
		o.pushed++
		r["status"] = "pushed"
	case errors.Is(err, chdomain.ErrNotImplemented):
		// Not a failure of this push: the adapter can't send yet. Counted
		// apart so a property selling only on live OTAs still reads "sent".
		o.skipped++
		r["status"] = "not_live"
	default:
		o.failed++
		r["status"] = "failed"
		r["error"] = err.Error()
		o.errors = append(o.errors, ch.Provider+": "+err.Error())
	}
	o.results = append(o.results, r)
}

func (o *pushOutcome) response() map[string]any {
	results := o.results
	if results == nil {
		results = []map[string]any{}
	}
	return map[string]any{
		"channels_pushed":  o.pushed,
		"channels_failed":  o.failed,
		"channels_skipped": o.skipped,
		"errors":           o.errors,
		"results":          results,
	}
}

// utcPtr renders an optional timestamp in UTC. pgx scans timestamptz into
// the process's local zone, so without this the API answered in whatever
// zone the server ran in (+05:30 on a dev Mac) while the PMS works in UTC.
func utcPtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}

// channelFilter limits a push or fetch to some of the property's channels:
// `only_channel_ids` in the request, for a Retry or Sync now on one channel
// (BUG-068). Absent or empty means every active channel.
type channelFilter map[string]bool

func parseChannelFilter(body map[string]any) channelFilter {
	raw, _ := body["only_channel_ids"].([]any)
	if len(raw) == 0 {
		return nil
	}
	f := channelFilter{}
	for _, v := range raw {
		if id, ok := v.(string); ok && id != "" {
			f[id] = true
		}
	}
	return f
}

func (f channelFilter) allows(channelID string) bool {
	return len(f) == 0 || f[channelID]
}
