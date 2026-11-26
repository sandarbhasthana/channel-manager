package usecases

import (
	"errors"
	"testing"
	"time"

	chdomain "github.com/channel-manager/channel-manager/services/channel/domain"
)

// BUG-058: a push that failed on one channel must say so per channel, and a
// placeholder adapter (Airbnb, Agoda) must read "not live", not "pushed".
func TestPushOutcomeReportsEachChannel(t *testing.T) {
	var o pushOutcome
	o.record(chdomain.Channel{ID: "c1", ConnectionID: "k1", Provider: "bookingcom"}, nil)
	o.record(chdomain.Channel{ID: "c2", ConnectionID: "k2", Provider: "expedia"}, errors.New("dial tcp: connection refused"))
	o.record(chdomain.Channel{ID: "c3", ConnectionID: "k3", Provider: "airbnb"}, chdomain.ErrNotImplemented)

	r := o.response()
	if r["channels_pushed"] != 1 || r["channels_failed"] != 1 || r["channels_skipped"] != 1 {
		t.Fatalf("counts = %v/%v/%v", r["channels_pushed"], r["channels_failed"], r["channels_skipped"])
	}
	results := r["results"].([]map[string]any)
	want := []struct{ provider, status, name string }{
		{"bookingcom", "pushed", "Booking.com"},
		{"expedia", "failed", "Expedia"},
		{"airbnb", "not_live", "Airbnb"},
	}
	for i, w := range want {
		if results[i]["provider"] != w.provider || results[i]["status"] != w.status || results[i]["name"] != w.name {
			t.Errorf("result %d = %v, want %+v", i, results[i], w)
		}
	}
	if results[1]["error"] != "dial tcp: connection refused" {
		t.Errorf("failed result lost its error: %v", results[1])
	}
	if _, ok := results[2]["error"]; ok {
		t.Errorf("not-live channel should carry no error: %v", results[2])
	}
}

func TestPushOutcomeEmptyResultsIsAnArray(t *testing.T) {
	var o pushOutcome
	if got := o.response()["results"]; got == nil {
		t.Fatal("results should be an empty array, not null")
	}
}

func TestUTCPtr(t *testing.T) {
	ist := time.FixedZone("IST", 5*3600+1800)
	in := time.Date(2026, 11, 24, 21, 21, 16, 0, ist)
	got := utcPtr(&in)
	if got.Location() != time.UTC || !got.Equal(in) || got.Hour() != 15 {
		t.Fatalf("utcPtr = %v", got)
	}
	if utcPtr(nil) != nil {
		t.Fatal("nil in, nil out")
	}
}

func TestChannelFilter(t *testing.T) {
	if f := parseChannelFilter(map[string]any{}); !f.allows("any") {
		t.Fatal("no filter should allow every channel")
	}
	f := parseChannelFilter(map[string]any{"only_channel_ids": []any{"c1", 7, ""}})
	if !f.allows("c1") || f.allows("c2") {
		t.Fatalf("filter = %v", f)
	}
}
