package main

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

type config struct {
	apiURL         string
	token          string
	pullEvery      time.Duration
	redeliverEvery time.Duration
}

type scheduledJob struct {
	name  string
	every time.Duration
}

func loadConfig(getenv func(string) string) (config, error) {
	c := config{
		apiURL:         strings.TrimRight(orDefault(getenv("SYNC_WORKER_API_URL"), "http://localhost:8080"), "/"),
		token:          getenv("SYNC_WORKER_TOKEN"),
		pullEvery:      5 * time.Minute,
		redeliverEvery: time.Minute,
	}
	if c.token == "" {
		return c, errors.New("SYNC_WORKER_TOKEN is required (the same value the API is started with)")
	}
	var err error
	if c.pullEvery, err = interval(getenv, "SYNC_PULL_INTERVAL", c.pullEvery); err != nil {
		return c, err
	}
	if c.redeliverEvery, err = interval(getenv, "SYNC_REDELIVER_INTERVAL", c.redeliverEvery); err != nil {
		return c, err
	}
	if c.pullEvery == 0 && c.redeliverEvery == 0 {
		return c, errors.New("both jobs are switched off (SYNC_PULL_INTERVAL and SYNC_REDELIVER_INTERVAL are 0)")
	}
	return c, nil
}

// schedule lists the jobs that are on. A pull also redelivers, so when both
// run, redelivery covers the minutes between pulls.
func (c config) schedule() []scheduledJob {
	var jobs []scheduledJob
	if c.pullEvery > 0 {
		jobs = append(jobs, scheduledJob{name: "pull_reservations", every: c.pullEvery})
	}
	if c.redeliverEvery > 0 {
		jobs = append(jobs, scheduledJob{name: "redeliver", every: c.redeliverEvery})
	}
	return jobs
}

// minInterval keeps a typo ("5s" for "5m") from hammering the OTAs.
const minInterval = 15 * time.Second

func interval(getenv func(string) string, key string, def time.Duration) (time.Duration, error) {
	v := strings.TrimSpace(getenv(key))
	if v == "" {
		return def, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	if d == 0 {
		return 0, nil
	}
	if d < minInterval {
		return 0, fmt.Errorf("%s: %s is shorter than the %s minimum", key, d, minInterval)
	}
	return d, nil
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func describe(jobs []scheduledJob) []string {
	out := make([]string, 0, len(jobs))
	for _, j := range jobs {
		out = append(out, fmt.Sprintf("%s every %s", j.name, j.every))
	}
	return out
}
