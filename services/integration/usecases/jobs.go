package usecases

import (
	"context"
	"fmt"
	"time"

	platformauth "github.com/channel-manager/channel-manager/platform/auth"
)

// Background jobs the sync worker triggers on a schedule. They run in the API
// process because that is where every service is wired and where OTA
// credentials are held; the worker (apps/sync-worker) only decides when.

const (
	// JobRedeliver resends OTA bookings the PMS didn't take when they were
	// first announced (it was down, or answered "try again").
	JobRedeliver = "redeliver"
	// JobPullReservations fetches new and changed bookings from every active
	// channel of every property and hands them to the PMS, so OTA bookings
	// arrive without anyone pressing Full sync. It also redelivers.
	JobPullReservations = "pull_reservations"
)

// Jobs lists the job names RunJob accepts.
var Jobs = []string{JobRedeliver, JobPullReservations}

// OrgLister lists the tenants to run a job for. The tenancy directory isn't
// row-level secured, so it is read outside any tenant.
type OrgLister interface {
	ActiveOrgIDs(ctx context.Context) ([]string, error)
}

// JobResult is what one run did, for the worker's log.
type JobResult struct {
	Job         string   `json:"job"`
	Orgs        int      `json:"orgs"`
	Properties  int      `json:"properties"`
	Imported    int      `json:"imported"`
	Redelivered int      `json:"redelivered"`
	Undelivered int      `json:"undelivered"`
	Errors      []string `json:"errors,omitempty"`
	DurationMs  int64    `json:"duration_ms"`
}

// perProperty bounds one property's share of a run, so one slow OTA can't
// stall every other tenant.
const perProperty = 90 * time.Second

// RunJob runs a job for every active property of every active tenant. A
// failure on one property is recorded and the run continues.
func (s *Service) RunJob(ctx context.Context, orgs OrgLister, job string) (JobResult, error) {
	if job != JobRedeliver && job != JobPullReservations {
		return JobResult{}, fmt.Errorf("unknown job %q", job)
	}
	started := time.Now()
	out := JobResult{Job: job}
	ids, err := orgs.ActiveOrgIDs(ctx)
	if err != nil {
		return out, fmt.Errorf("list tenants: %w", err)
	}
	for _, orgID := range ids {
		if ctx.Err() != nil {
			break
		}
		out.Orgs++
		tctx := platformauth.WithTenantContext(ctx, platformauth.TenantContext{
			UserID: "system:sync-worker",
			OrgID:  orgID,
			Role:   "integration",
		})
		props, err := s.props.ListByOrg(tctx, orgID)
		if err != nil {
			out.Errors = append(out.Errors, fmt.Sprintf("org %s: list properties: %v", orgID, err))
			continue
		}
		for _, p := range props {
			if !p.IsActive {
				continue
			}
			out.Properties++
			pctx, cancel := context.WithTimeout(tctx, perProperty)
			switch job {
			case JobRedeliver:
				d, u, err := s.res.RedeliverPending(pctx, p.ID)
				out.Redelivered += d
				out.Undelivered += u
				if err != nil {
					out.Errors = append(out.Errors, fmt.Sprintf("property %s: %v", p.ID, err))
				}
			case JobPullReservations:
				res, err := s.fetchChannelReservations(pctx, p.ID, p.ExternalID, map[string]any{})
				if err != nil {
					out.Errors = append(out.Errors, fmt.Sprintf("property %s: %v", p.ID, err))
				} else {
					out.Imported += intOf(res["ingested"])
					out.Redelivered += intOf(res["redelivered"])
					out.Undelivered += intOf(res["undelivered"])
					if fe, ok := res["fetch_errors"].([]string); ok {
						for _, e := range fe {
							out.Errors = append(out.Errors, fmt.Sprintf("property %s: %s", p.ID, e))
						}
					}
				}
			}
			cancel()
		}
	}
	out.DurationMs = time.Since(started).Milliseconds()
	return out, nil
}

func intOf(v any) int {
	if n, ok := v.(int); ok {
		return n
	}
	return 0
}
