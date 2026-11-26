package domain

import (
	"encoding/json"
	"errors"
)

// Delivery is whether the PMS has taken a booking this channel manager
// announced, kept in the reservation's metadata under "pms_delivery".
//
// A booking is announced to the PMS once. If the PMS was unreachable or
// failed at that moment, the booking used to be lost: the next sync found it
// already stored and unchanged, so nothing resent it. A pending delivery
// keeps a copy of what was sent and is retried until the PMS takes it.
type Delivery struct {
	State     string       `json:"state"` // DeliveryPending | DeliveryDelivered
	Attempts  int          `json:"attempts,omitempty"`
	LastError string       `json:"last_error,omitempty"`
	Booking   *Reservation `json:"booking,omitempty"`
}

const (
	DeliveryPending   = "pending"
	DeliveryDelivered = "delivered"
	deliveryKey       = "pms_delivery"
)

// ErrPMSUnavailable wraps a publish the PMS did not take (unreachable, or a
// response that means "try again"). The booking stays pending and is resent.
var ErrPMSUnavailable = errors.New("reservations: PMS did not take the booking")

// DeliveryOf reads the delivery state from a reservation's metadata. A
// booking stored before delivery was tracked reads as delivered.
func DeliveryOf(metadata []byte) Delivery {
	var m map[string]json.RawMessage
	if len(metadata) == 0 || json.Unmarshal(metadata, &m) != nil {
		return Delivery{State: DeliveryDelivered}
	}
	raw, ok := m[deliveryKey]
	if !ok {
		return Delivery{State: DeliveryDelivered}
	}
	var d Delivery
	if json.Unmarshal(raw, &d) != nil || d.State == "" {
		return Delivery{State: DeliveryDelivered}
	}
	return d
}

// WithPendingDelivery returns metadata marked pending, carrying a copy of the
// booking to resend. Other metadata keys are kept.
func WithPendingDelivery(metadata []byte, booking Reservation) ([]byte, error) {
	m := map[string]any{}
	if len(metadata) > 0 {
		if err := json.Unmarshal(metadata, &m); err != nil {
			m = map[string]any{}
		}
	}
	prev := DeliveryOf(metadata)
	booking.RawPayload = nil // the copy must not nest the metadata it lives in
	m[deliveryKey] = Delivery{State: DeliveryPending, Attempts: prev.Attempts, LastError: prev.LastError, Booking: &booking}
	return json.Marshal(m)
}
