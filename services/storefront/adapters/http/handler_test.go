package http

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/channel-manager/channel-manager/services/pms/adapters/mypms"
	"github.com/channel-manager/channel-manager/services/storefront/domain"
)

func TestStatusForPreservesUpstreamPMSNotFoundAndConflict(t *testing.T) {
	notFound := fmt.Errorf("storefront: cancel booking: %w", &mypms.APIError{
		StatusCode: http.StatusNotFound,
		Message:    `{"error":"Booking not found"}`,
	})
	if got := statusFor(notFound); got != http.StatusNotFound {
		t.Fatalf("not found status = %d", got)
	}

	conflict := fmt.Errorf("storefront: create booking: %w", &mypms.APIError{
		StatusCode: http.StatusConflict,
		Message:    `{"error":"Room is already booked for the selected dates"}`,
	})
	if got := statusFor(conflict); got != http.StatusConflict {
		t.Fatalf("conflict status = %d", got)
	}
}

func TestStatusForKeepsDomainConflicts(t *testing.T) {
	if got := statusFor(domain.ErrDuplicateRequest); got != http.StatusConflict {
		t.Fatalf("duplicate request status = %d", got)
	}
	if got := statusFor(fmt.Errorf("unknown")); got != http.StatusBadRequest {
		t.Fatalf("unknown status = %d", got)
	}
}
