package usecases

import (
	"context"
	"errors"
	"sync"
	"time"

	platformauth "github.com/channel-manager/channel-manager/platform/auth"
	pmsdomain "github.com/channel-manager/channel-manager/services/pms/domain"
	pricingdomain "github.com/channel-manager/channel-manager/services/pricing/domain"
	resdomain "github.com/channel-manager/channel-manager/services/reservations/domain"
	"github.com/channel-manager/channel-manager/services/storefront/domain"
	"github.com/channel-manager/channel-manager/services/storefront/ports"
)

const (
	testOrgID     = "11111111-1111-1111-1111-111111111111"
	testPropID    = "22222222-2222-2222-2222-222222222222"
	testExtPropID = "ext-prop-1"
	testRoomID    = "room-101"
)

// tenantCtx returns a context carrying the tenant context the service expects.
func tenantCtx() context.Context {
	return platformauth.WithTenantContext(context.Background(), platformauth.TenantContext{
		UserID: "user-1",
		OrgID:  testOrgID,
		Role:   "admin",
	})
}

// ── property lookup ─────────────────────────────────────────────────────────

type fakeProps struct {
	err        error
	beDisabled bool // when true, BookingEngineEnabled reports false
}

func (f *fakeProps) BookingEngineEnabled(_ context.Context, _ string) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	return !f.beDisabled, nil
}

func (f *fakeProps) GetChannelConfig(_ context.Context, _ string) (pmsdomain.ChannelConfig, error) {
	if f.err != nil {
		return pmsdomain.ChannelConfig{}, f.err
	}
	return pmsdomain.ChannelConfig{
		Enabled: !f.beDisabled,
		Route:   "pms",
		Percent: 0,
	}, nil
}

func (f *fakeProps) GetByID(_ context.Context, id string) (pmsdomain.Property, error) {
	if f.err != nil {
		return pmsdomain.Property{}, f.err
	}
	return pmsdomain.Property{
		ID: testPropID, OrgID: testOrgID, ConnectionID: "conn-1",
		ExternalID: testExtPropID, Name: "Test Hotel", DefaultCurrency: "USD",
	}, nil
}

func (f *fakeProps) GetByExternalID(_ context.Context, _, _ string) (pmsdomain.Property, error) {
	return pmsdomain.Property{}, errors.New("not found")
}

func (f *fakeProps) ListListings(_ context.Context) ([]pmsdomain.PropertyListing, error) {
	if f.err != nil {
		return nil, f.err
	}
	return []pmsdomain.PropertyListing{{
		ID: testPropID, ExternalID: testExtPropID, Name: "Test Hotel",
		Timezone: "UTC", DefaultCurrency: "USD", IsActive: true, IsDefault: true,
		Channel: pmsdomain.ChannelConfig{Enabled: !f.beDisabled, Route: "pms"},
	}}, nil
}

// ── PMS gateway ─────────────────────────────────────────────────────────────

type fakePms struct {
	offers        []pmsdomain.AvailabilityOffer
	quote         *pmsdomain.Quote
	flexible      *pmsdomain.FlexibleAvailabilityResult
	flexibleQuery pmsdomain.FlexibleAvailabilityQuery

	createErr    error
	createCalls  int
	createdInput pmsdomain.CreateBookingInput
	booking      *pmsdomain.PmsBooking
	cancelResult *pmsdomain.CancelBookingResult
	cancelErr    error
	cancelCalls  int
}

func (f *fakePms) SearchAvailability(_ context.Context, _ string, _ pmsdomain.AvailabilityQuery) ([]pmsdomain.AvailabilityOffer, error) {
	return f.offers, nil
}

func (f *fakePms) SearchFlexibleAvailability(_ context.Context, _ string, q pmsdomain.FlexibleAvailabilityQuery) (*pmsdomain.FlexibleAvailabilityResult, error) {
	f.flexibleQuery = q
	if f.flexible != nil {
		return f.flexible, nil
	}
	return &pmsdomain.FlexibleAvailabilityResult{
		Nights:         q.Nights,
		Adults:         q.Adults,
		Children:       q.Children,
		RequestedRooms: q.Rooms,
		SortBy:         q.SortBy,
		Stays: []pmsdomain.FlexibleStay{{
			Checkin:        "2026-08-16",
			Checkout:       "2026-08-18",
			Nights:         q.Nights,
			CanAccommodate: true,
			Offers:         f.offers,
			TotalAvailable: len(f.offers),
		}},
		TotalMatching: 1,
		Returned:      1,
	}, nil
}

func (f *fakePms) GetQuote(_ context.Context, _ string, _ pmsdomain.QuoteQuery) (*pmsdomain.Quote, error) {
	if f.quote == nil {
		return nil, errors.New("no quote configured")
	}
	return f.quote, nil
}

func (f *fakePms) CreateBooking(_ context.Context, _ string, in pmsdomain.CreateBookingInput) (*pmsdomain.PmsBooking, error) {
	f.createCalls++
	f.createdInput = in
	if f.createErr != nil {
		return nil, f.createErr
	}
	if f.booking != nil {
		return f.booking, nil
	}
	return &pmsdomain.PmsBooking{
		BookingIDs: []string{"pms-booking-1"}, RoomIDs: append([]string(nil), in.RoomIDs...),
		RoomNames: []string{"101"}, RoomTypes: []string{"Deluxe"},
		GroupStatus: "CONFIRMED", Status: "CONFIRMED",
		GuestName: in.GuestName, Adults: in.Adults, Children: in.Children, PaymentStatus: "PAID",
	}, nil
}

func (f *fakePms) GetBooking(_ context.Context, _ string, in pmsdomain.GetBookingInput) (*pmsdomain.PmsBooking, error) {
	return &pmsdomain.PmsBooking{BookingID: in.BookingID, Status: "CONFIRMED"}, nil
}

func (f *fakePms) UpdateBooking(_ context.Context, _ string, in pmsdomain.UpdateBookingInput) (*pmsdomain.PmsBooking, error) {
	return &pmsdomain.PmsBooking{
		BookingID: in.BookingID,
		Status:    "CONFIRMATION_PENDING",
		GuestName: in.GuestName,
		Message:   "updated",
	}, nil
}

func (f *fakePms) CancelBooking(_ context.Context, _ string, in pmsdomain.CancelBookingInput) (*pmsdomain.CancelBookingResult, error) {
	f.cancelCalls++
	if f.cancelErr != nil {
		return nil, f.cancelErr
	}
	if f.cancelResult != nil {
		return f.cancelResult, nil
	}
	return &pmsdomain.CancelBookingResult{BookingID: in.BookingID, Status: "CANCELLED"}, nil
}

// ── reservation writer ──────────────────────────────────────────────────────

type fakeRes struct {
	ingestErr    error
	ingestCalls  int
	ingested     *resdomain.Reservation
	cancelCalls  int
	cancelledIDs []string
}

func (f *fakeRes) IngestReservation(_ context.Context, res *resdomain.Reservation, _ string) (string, bool, error) {
	f.ingestCalls++
	if f.ingestErr != nil {
		return "", false, f.ingestErr
	}
	f.ingested = res
	return "reservation-1", true, nil
}

// RecordReservation is the non-publishing persist the storefront now uses for
// direct bookings; for the fakes it behaves identically to IngestReservation.
func (f *fakeRes) RecordReservation(_ context.Context, res *resdomain.Reservation, _ string) (string, bool, error) {
	f.ingestCalls++
	if f.ingestErr != nil {
		return "", false, f.ingestErr
	}
	f.ingested = res
	return "reservation-1", true, nil
}

func (f *fakeRes) CancelReservation(_ context.Context, id string) (*resdomain.Reservation, error) {
	f.cancelCalls++
	f.cancelledIDs = append(f.cancelledIDs, id)
	return &resdomain.Reservation{ID: id, Status: "cancelled"}, nil
}

// ── hold store ──────────────────────────────────────────────────────────────

type fakeHolds struct {
	mu            sync.Mutex
	holds         map[string]domain.Hold
	placeErr      error
	releaseCalls  int
	releasedToken []string
}

func newFakeHolds() *fakeHolds {
	return &fakeHolds{holds: make(map[string]domain.Hold)}
}

func (f *fakeHolds) Place(_ context.Context, h domain.Hold) error {
	if f.placeErr != nil {
		return f.placeErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.holds[h.Token] = h
	return nil
}

func (f *fakeHolds) Get(_ context.Context, token string) (domain.Hold, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	h, ok := f.holds[token]
	if !ok {
		return domain.Hold{}, domain.ErrHoldNotFound
	}
	return h, nil
}

func (f *fakeHolds) Release(_ context.Context, token string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.releaseCalls++
	f.releasedToken = append(f.releasedToken, token)
	delete(f.holds, token)
	return nil
}

func (f *fakeHolds) ActiveForProperty(_ context.Context, propertyID string) ([]domain.Hold, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []domain.Hold
	for _, h := range f.holds {
		if h.PropertyID == propertyID {
			out = append(out, h)
		}
	}
	return out, nil
}

// seed inserts a live hold directly, bypassing Place.
func (f *fakeHolds) seed(h domain.Hold) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.holds[h.Token] = h
}

type fakeOffers struct{ offers map[string]domain.Offer }

func newFakeOffers() *fakeOffers { return &fakeOffers{offers: make(map[string]domain.Offer)} }

func (f *fakeOffers) Put(_ context.Context, offer domain.Offer) error {
	f.offers[offer.ID] = offer
	return nil
}

func (f *fakeOffers) Get(_ context.Context, id string) (domain.Offer, error) {
	offer, ok := f.offers[id]
	if !ok || !offer.ExpiresAt.After(time.Now()) {
		return domain.Offer{}, domain.ErrOfferNotFound
	}
	return offer, nil
}

// ── idempotency store ───────────────────────────────────────────────────────

type fakeIdem struct {
	records  map[string]ports.IdempotencyRecord
	putCalls int
}

func newFakeIdem() *fakeIdem {
	return &fakeIdem{records: make(map[string]ports.IdempotencyRecord)}
}

func (f *fakeIdem) Get(_ context.Context, key string) (ports.IdempotencyRecord, bool, error) {
	record, ok := f.records[key]
	return record, ok, nil
}

func (f *fakeIdem) Put(_ context.Context, key string, record ports.IdempotencyRecord) error {
	f.putCalls++
	f.records[key] = record
	return nil
}

// ── promo gateway ───────────────────────────────────────────────────────────

type fakePromos struct {
	promo pricingdomain.PromoCode

	lookupErr    error
	lookupReason error

	redeemErr    error
	redeemCalls  int
	releaseCalls int
	releasedCode []string
}

func (f *fakePromos) Lookup(_ context.Context, code, propertyID string) (pricingdomain.LookupResult, error) {
	if f.lookupErr != nil {
		return pricingdomain.LookupResult{}, f.lookupErr
	}
	if f.lookupReason != nil {
		return pricingdomain.LookupResult{Promo: f.promo, Valid: false, Reason: f.lookupReason}, nil
	}
	return pricingdomain.LookupResult{Promo: f.promo, Valid: true}, nil
}

func (f *fakePromos) Redeem(_ context.Context, code, propertyID string) (pricingdomain.PromoCode, error) {
	f.redeemCalls++
	if f.redeemErr != nil {
		return pricingdomain.PromoCode{}, f.redeemErr
	}
	p := f.promo
	p.Uses++
	return p, nil
}

func (f *fakePromos) ReleaseRedemption(_ context.Context, code string) error {
	f.releaseCalls++
	f.releasedCode = append(f.releasedCode, code)
	return nil
}

// ── audit recorder ──────────────────────────────────────────────────────────

type fakeAudit struct{ events []ports.AuditEvent }

func (f *fakeAudit) Record(_ context.Context, e ports.AuditEvent) {
	f.events = append(f.events, e)
}

func (f *fakeAudit) actions() []string {
	out := make([]string, 0, len(f.events))
	for _, e := range f.events {
		out = append(out, e.Action)
	}
	return out
}

// ── harness ─────────────────────────────────────────────────────────────────

type harness struct {
	svc    *Service
	props  *fakeProps
	pms    *fakePms
	res    *fakeRes
	promos *fakePromos
	holds  *fakeHolds
	offers *fakeOffers
	idem   *fakeIdem
	audit  *fakeAudit
}

func newHarness() *harness {
	h := &harness{
		props: &fakeProps{},
		pms:   &fakePms{},
		res:   &fakeRes{},
		promos: &fakePromos{
			promo: pricingdomain.PromoCode{Code: "SUMMER25", DiscountPct: 25, IsActive: true},
		},
		holds:  newFakeHolds(),
		offers: newFakeOffers(),
		idem:   newFakeIdem(),
		audit:  &fakeAudit{},
	}
	h.svc = NewService(h.props, h.pms, h.res, h.promos, h.holds, h.offers, h.idem, h.audit, 10*time.Minute)
	return h
}

// liveHold seeds an unexpired hold for the test property/room.
func (h *harness) liveHold(token string) domain.Hold {
	hold := domain.Hold{
		Token:      token,
		PropertyID: testPropID,
		RoomID:     testRoomID,
		RoomTypeID: "Deluxe",
		Checkin:    mustDay("2026-08-01"),
		Checkout:   mustDay("2026-08-03"),
		ExpiresAt:  time.Now().Add(5 * time.Minute),
	}
	h.holds.seed(hold)
	return hold
}

func mustDay(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

func createBody(extra map[string]any) map[string]any {
	body := map[string]any{
		"checkin":         "2026-08-01",
		"checkout":        "2026-08-03",
		"room_ids":        []any{testRoomID},
		"guest_name":      "Ada Lovelace",
		"email":           "ada@example.com",
		"adults":          float64(2),
		"total_amount":    float64(450),
		"currency":        "USD",
		"idempotency_key": "idem-default",
	}
	for k, v := range extra {
		body[k] = v
	}
	return body
}
