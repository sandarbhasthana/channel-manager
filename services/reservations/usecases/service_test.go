package usecases

import (
	"context"
	"fmt"
	"testing"

	"github.com/channel-manager/channel-manager/services/reservations/domain"
)

// fakeRepo stores by external id the way the postgres repo does: new,
// changed, unchanged, and "never taken by the PMS" (pending).
type fakeRepo struct {
	byExt map[string]*row
}

type row struct {
	res       domain.Reservation
	delivered bool
}

func newFakeRepo() *fakeRepo { return &fakeRepo{byExt: map[string]*row{}} }

func (f *fakeRepo) GetByID(context.Context, string) (*domain.Reservation, error) { return nil, nil }
func (f *fakeRepo) ListByProperty(context.Context, string) ([]domain.Reservation, error) {
	return nil, nil
}
func (f *fakeRepo) UpdateStatus(context.Context, string, string) error { return nil }

func (f *fakeRepo) Save(_ context.Context, res *domain.Reservation) (domain.SaveResult, error) {
	r, ok := f.byExt[res.ChannelConfirmationID]
	if !ok {
		res.ID = "id-" + res.ChannelConfirmationID
		f.byExt[res.ChannelConfirmationID] = &row{res: *res, delivered: !res.AwaitPMS}
		return domain.SaveInserted, nil
	}
	res.ID = r.res.ID
	same := r.res.SameStay(*res)
	switch {
	case same && r.delivered:
		return domain.SaveUnchanged, nil
	case !r.delivered:
		r.res = *res
		return domain.SaveInserted, nil
	default:
		r.res = *res
		if res.AwaitPMS {
			r.delivered = false
		}
		return domain.SaveUpdated, nil
	}
}

func (f *fakeRepo) MarkDelivery(_ context.Context, id string, delivered bool, _ string) error {
	for _, r := range f.byExt {
		if r.res.ID == id {
			r.delivered = delivered
		}
	}
	return nil
}

func (f *fakeRepo) ListUndelivered(context.Context, string) ([]domain.Reservation, error) {
	var out []domain.Reservation
	for _, r := range f.byExt {
		if !r.delivered {
			out = append(out, r.res)
		}
	}
	return out, nil
}

// fakePMS takes bookings only while up.
type fakePMS struct {
	up      bool
	created []string
	updated []string
}

func (p *fakePMS) PublishReservationCreated(_ context.Context, r *domain.Reservation) error {
	if !p.up {
		return fmt.Errorf("%w: connection refused", domain.ErrPMSUnavailable)
	}
	p.created = append(p.created, r.ChannelConfirmationID)
	return nil
}

func (p *fakePMS) PublishReservationUpdated(_ context.Context, r *domain.Reservation) error {
	if !p.up {
		return fmt.Errorf("%w: connection refused", domain.ErrPMSUnavailable)
	}
	p.updated = append(p.updated, r.ChannelConfirmationID)
	return nil
}

func booking() *domain.Reservation {
	return &domain.Reservation{PropertyID: "p1", ChannelID: "conn", ChannelConfirmationID: "BK-1", Status: "confirmed", TotalAmount: 250}
}

func TestBookingIsResentAfterThePMSWasDown(t *testing.T) {
	repo, pms := newFakeRepo(), &fakePMS{up: false}
	svc := NewReservationService(repo, pms)
	ctx := context.Background()

	if _, _, err := svc.IngestReservation(ctx, booking(), ""); err != nil {
		t.Fatal(err)
	}
	if len(pms.created) != 0 {
		t.Fatal("PMS was down; nothing should have been taken")
	}

	// The PMS is back. The same booking, unchanged, is re-read on the next
	// sync: it must go again, because the PMS never had it.
	pms.up = true
	if _, _, err := svc.IngestReservation(ctx, booking(), ""); err != nil {
		t.Fatal(err)
	}
	if len(pms.created) != 1 {
		t.Fatalf("created = %v, want the booking resent once", pms.created)
	}

	// Taken now: a third read is a no-op.
	if _, _, err := svc.IngestReservation(ctx, booking(), ""); err != nil {
		t.Fatal(err)
	}
	if len(pms.created) != 1 || len(pms.updated) != 0 {
		t.Fatalf("created %v updated %v after delivery", pms.created, pms.updated)
	}
}

func TestRedeliverPendingSendsWhatThePMSMissed(t *testing.T) {
	repo, pms := newFakeRepo(), &fakePMS{up: false}
	svc := NewReservationService(repo, pms)
	ctx := context.Background()
	_, _, _ = svc.IngestReservation(ctx, booking(), "")

	if d, p, _ := svc.RedeliverPending(ctx, "p1"); d != 0 || p != 1 {
		t.Fatalf("while down: delivered %d pending %d", d, p)
	}
	pms.up = true
	if d, p, _ := svc.RedeliverPending(ctx, "p1"); d != 1 || p != 0 {
		t.Fatalf("after recovery: delivered %d pending %d", d, p)
	}
	if d, p, _ := svc.RedeliverPending(ctx, "p1"); d != 0 || p != 0 {
		t.Fatalf("nothing should be left: delivered %d pending %d", d, p)
	}
}

func TestRecordedBookingsAreNotAnnounced(t *testing.T) {
	repo, pms := newFakeRepo(), &fakePMS{up: true}
	svc := NewReservationService(repo, pms)
	_, _, _ = svc.RecordReservation(context.Background(), booking(), "")
	if len(pms.created) != 0 {
		t.Fatal("a booking the PMS made must not be announced back")
	}
	if d, p, _ := svc.RedeliverPending(context.Background(), "p1"); d+p != 0 {
		t.Fatal("a recorded booking is never pending")
	}
}
