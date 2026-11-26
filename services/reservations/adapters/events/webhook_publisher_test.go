package events

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/channel-manager/channel-manager/services/reservations/domain"
)

// The PMS files an OTA booking under ota_reference, so it must be the
// channel's name ("Booking.com"), with the connector id on its own field.
// Sending the connector id as ota_reference made the PMS create a channel
// named after the uuid (BUG-056).
func TestWebhookCarriesChannelNameAndConnectorSeparately(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &got)
	}))
	defer srv.Close()

	res := &domain.Reservation{
		PropertyID:            "prop-1",
		ChannelID:             "c6120212-a18a-4659-9215-f68ee5df4f8a",
		ChannelProvider:       "bookingcom",
		ChannelName:           "Booking.com",
		ChannelConfirmationID: "BK-1",
		CheckIn:               time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC),
		CheckOut:              time.Date(2026, 12, 3, 0, 0, 0, 0, time.UTC),
	}
	if err := NewWebhookPublisher(srv.URL).PublishReservationCreated(context.Background(), res); err != nil {
		t.Fatal(err)
	}
	data, _ := got["data"].(map[string]any)
	if data["ota_reference"] != "Booking.com" {
		t.Errorf("ota_reference = %v, want Booking.com", data["ota_reference"])
	}
	if data["channel_provider"] != "bookingcom" {
		t.Errorf("channel_provider = %v", data["channel_provider"])
	}
	if data["channel_connection_id"] != res.ChannelID {
		t.Errorf("channel_connection_id = %v", data["channel_connection_id"])
	}
}

func TestOTAReferenceFallsBack(t *testing.T) {
	cases := []struct {
		res  domain.Reservation
		want string
	}{
		{domain.Reservation{ChannelName: "Expedia", ChannelProvider: "expedia", ChannelID: "u"}, "Expedia"},
		{domain.Reservation{ChannelProvider: "expedia", ChannelID: "u"}, "expedia"},
		{domain.Reservation{ChannelID: "u"}, "u"},
	}
	for _, c := range cases {
		if got := otaReference(&c.res); got != c.want {
			t.Errorf("otaReference = %q, want %q", got, c.want)
		}
	}
}

// A booking the PMS didn't take must come back as an error, so it is kept
// pending and resent; a 422 (recorded as unimportable) must not.
func TestWebhookReportsWhetherThePMSTookIt(t *testing.T) {
	for _, tc := range []struct {
		status    int
		retryable bool
	}{
		{200, false}, {422, false}, {400, false},
		{500, true}, {502, true}, {503, true}, {429, true}, {404, true}, {401, true},
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tc.status)
		}))
		err := NewWebhookPublisher(srv.URL).PublishReservationCreated(context.Background(), &domain.Reservation{})
		srv.Close()
		if got := errors.Is(err, domain.ErrPMSUnavailable); got != tc.retryable {
			t.Errorf("status %d: retryable = %v, want %v (err %v)", tc.status, got, tc.retryable, err)
		}
	}

	// PMS not listening at all.
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close()
	if err := NewWebhookPublisher(url).PublishReservationCreated(context.Background(), &domain.Reservation{}); !errors.Is(err, domain.ErrPMSUnavailable) {
		t.Errorf("unreachable PMS: err = %v", err)
	}
}
