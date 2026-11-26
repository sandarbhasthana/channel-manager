package main

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestConfig(t *testing.T) {
	if _, err := loadConfig(env(nil)); err == nil {
		t.Fatal("a missing token must stop the worker")
	}
	c, err := loadConfig(env(map[string]string{"SYNC_WORKER_TOKEN": "t"}))
	if err != nil {
		t.Fatal(err)
	}
	if c.apiURL != "http://localhost:8080" || c.pullEvery != 5*time.Minute || c.redeliverEvery != time.Minute {
		t.Errorf("defaults = %+v", c)
	}
	if got := describe(c.schedule()); len(got) != 2 {
		t.Errorf("schedule = %v", got)
	}
	c, _ = loadConfig(env(map[string]string{"SYNC_WORKER_TOKEN": "t", "SYNC_PULL_INTERVAL": "0", "SYNC_WORKER_API_URL": "http://api:8080/"}))
	if len(c.schedule()) != 1 || c.schedule()[0].name != "redeliver" || c.apiURL != "http://api:8080" {
		t.Errorf("pull off: %+v %v", c, c.schedule())
	}
	if _, err := loadConfig(env(map[string]string{"SYNC_WORKER_TOKEN": "t", "SYNC_PULL_INTERVAL": "5s"})); err == nil {
		t.Error("a 5s pull should be refused")
	}
	if _, err := loadConfig(env(map[string]string{"SYNC_WORKER_TOKEN": "t", "SYNC_PULL_INTERVAL": "0", "SYNC_REDELIVER_INTERVAL": "0"})); err == nil {
		t.Error("everything off should be refused")
	}
}

func TestClientCallsTheJobEndpointWithTheToken(t *testing.T) {
	var gotPath, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAuth = r.URL.Path, r.Header.Get("Authorization")
		_, _ = io.WriteString(w, `{"job":"redeliver","orgs":2,"properties":3,"redelivered":1}`)
	}))
	defer srv.Close()
	res, err := newJobClient(srv.URL, "s3cret").Run(context.Background(), "redeliver")
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/internal/worker/jobs/redeliver" || gotAuth != "Bearer s3cret" {
		t.Errorf("path %q auth %q", gotPath, gotAuth)
	}
	if res.Orgs != 2 || res.Properties != 3 || res.Redelivered != 1 {
		t.Errorf("result = %+v", res)
	}
}

func TestClientExplainsARefusedToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"error":"unauthorized"}`)
	}))
	defer srv.Close()
	_, err := newJobClient(srv.URL, "x").Run(context.Background(), "redeliver")
	if err == nil || !strings.Contains(err.Error(), "SYNC_WORKER_TOKEN") {
		t.Errorf("err = %v", err)
	}
}

type countingRunner struct {
	mu   sync.Mutex
	runs map[string]int
}

func (c *countingRunner) Run(_ context.Context, job string) (jobResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.runs[job]++
	return jobResult{Job: job}, nil
}

func TestRunStartsEachJobAndStopsOnCancel(t *testing.T) {
	r := &countingRunner{runs: map[string]int{}}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		run(ctx, slog.New(slog.NewTextHandler(io.Discard, nil)), r, []scheduledJob{
			{name: "pull_reservations", every: time.Hour},
			{name: "redeliver", every: 10 * time.Millisecond},
		})
		close(done)
	}()
	time.Sleep(60 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("worker didn't stop on cancel")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.runs["pull_reservations"] != 1 {
		t.Errorf("pull ran %d times, want once (it runs at start, then hourly)", r.runs["pull_reservations"])
	}
	if r.runs["redeliver"] < 3 {
		t.Errorf("redeliver ran %d times, want it repeated", r.runs["redeliver"])
	}
}
