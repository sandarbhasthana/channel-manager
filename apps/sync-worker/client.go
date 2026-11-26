package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// jobResult mirrors the API's usecases.JobResult.
type jobResult struct {
	Job         string   `json:"job"`
	Orgs        int      `json:"orgs"`
	Properties  int      `json:"properties"`
	Imported    int      `json:"imported"`
	Redelivered int      `json:"redelivered"`
	Undelivered int      `json:"undelivered"`
	Errors      []string `json:"errors"`
	DurationMs  int64    `json:"duration_ms"`
}

type jobRunner interface {
	Run(ctx context.Context, job string) (jobResult, error)
}

type jobClient struct {
	base  string
	token string
	http  *http.Client
}

func newJobClient(base, token string) *jobClient {
	// A pull visits every property's OTAs; give it room, but not forever.
	return &jobClient{base: base, token: token, http: &http.Client{Timeout: 10 * time.Minute}}
}

func (c *jobClient) Run(ctx context.Context, job string) (jobResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/internal/worker/jobs/"+job, nil)
	if err != nil {
		return jobResult{}, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	resp, err := c.http.Do(req)
	if err != nil {
		return jobResult{}, fmt.Errorf("API unreachable: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		var e struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(body, &e)
		if resp.StatusCode == http.StatusUnauthorized {
			return jobResult{}, fmt.Errorf("API refused the token (is SYNC_WORKER_TOKEN the same on both?): %s", e.Error)
		}
		return jobResult{}, fmt.Errorf("API answered %d: %s", resp.StatusCode, e.Error)
	}
	var res jobResult
	if err := json.Unmarshal(body, &res); err != nil {
		return jobResult{}, fmt.Errorf("unreadable reply: %w", err)
	}
	return res, nil
}
