package domain

import (
	"encoding/json"
	"testing"
	"time"
)

func TestDeliveryRoundTrip(t *testing.T) {
	if d := DeliveryOf(nil); d.State != DeliveryDelivered {
		t.Fatalf("untracked booking should read as delivered, got %q", d.State)
	}
	if d := DeliveryOf([]byte(`{"ota":"x"}`)); d.State != DeliveryDelivered {
		t.Fatalf("metadata without delivery should read as delivered, got %q", d.State)
	}

	b := Reservation{
		ChannelConfirmationID: "yn5hb9",
		ChannelProvider:       "bookingcom",
		ChannelName:           "Booking.com",
		ExternalPropertyID:    "prop-ext",
		GuestName:             "QA Unmapped Test",
		CheckIn:               time.Date(2026, 12, 20, 0, 0, 0, 0, time.UTC),
		CheckOut:              time.Date(2026, 12, 21, 0, 0, 0, 0, time.UTC),
		RawPayload:            json.RawMessage(`{"big":"payload"}`),
	}
	meta, err := WithPendingDelivery([]byte(`{"ota":"x"}`), b)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	_ = json.Unmarshal(meta, &m)
	if m["ota"] != "x" {
		t.Errorf("other metadata lost: %s", meta)
	}
	d := DeliveryOf(meta)
	if d.State != DeliveryPending || d.Booking == nil {
		t.Fatalf("delivery = %+v", d)
	}
	// What the resend needs to rebuild the webhook.
	got := d.Booking
	if got.ChannelConfirmationID != "yn5hb9" || got.ChannelName != "Booking.com" || got.ExternalPropertyID != "prop-ext" || got.GuestName != "QA Unmapped Test" {
		t.Errorf("booking copy = %+v", got)
	}
	if !got.CheckIn.Equal(b.CheckIn) {
		t.Errorf("check-in = %v", got.CheckIn)
	}
	if got.RawPayload != nil {
		t.Errorf("copy should not nest the metadata: %s", got.RawPayload)
	}
}
