package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	platformauth "github.com/channel-manager/channel-manager/platform/auth"
	platformdb "github.com/channel-manager/channel-manager/platform/db"
	"github.com/channel-manager/channel-manager/services/reservations/adapters/postgres/pgstore"
	"github.com/channel-manager/channel-manager/services/reservations/domain"
)

// Repository implements ports.ReservationRepository.
type Repository struct {
	pool *platformdb.Pool
}

// NewRepository creates a new reservation repository.
func NewRepository(pool *platformdb.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) GetByID(ctx context.Context, id string) (*domain.Reservation, error) {
	tc, err := platformauth.FromContext(ctx)
	if err != nil {
		return nil, err
	}
	rid, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("reservations: invalid id: %w", err)
	}
	var out *domain.Reservation
	err = r.pool.WithTenant(ctx, tc.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := pgstore.New(tx).GetReservation(ctx, rid)
		if err != nil {
			return err
		}
		res := mapGetRow(row)
		out = &res
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("reservations: get: %w", err)
	}
	return out, nil
}

func (r *Repository) ListByProperty(ctx context.Context, propertyID string) ([]domain.Reservation, error) {
	tc, err := platformauth.FromContext(ctx)
	if err != nil {
		return nil, err
	}
	pid, err := uuid.Parse(propertyID)
	if err != nil {
		return nil, fmt.Errorf("reservations: invalid property_id: %w", err)
	}
	var out []domain.Reservation
	err = r.pool.WithTenant(ctx, tc.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := pgstore.New(tx).ListReservationsByProperty(ctx, pid)
		if err != nil {
			return err
		}
		out = make([]domain.Reservation, 0, len(rows))
		for _, row := range rows {
			out = append(out, mapListRow(row))
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("reservations: list: %w", err)
	}
	return out, nil
}

func (r *Repository) Save(ctx context.Context, res *domain.Reservation) (domain.SaveResult, error) {
	tc, err := platformauth.FromContext(ctx)
	if err != nil {
		return domain.SaveInserted, err
	}
	if res.ID == "" {
		res.ID = uuid.NewString()
	}
	orgID, _ := uuid.Parse(tc.OrgID)
	propID, err := uuid.Parse(res.PropertyID)
	if err != nil {
		return domain.SaveInserted, fmt.Errorf("reservations: invalid property_id: %w", err)
	}
	resID, _ := uuid.Parse(res.ID)

	var connID pgtype.UUID
	if res.ChannelID != "" {
		if cid, err := uuid.Parse(res.ChannelID); err == nil {
			connID = pgtype.UUID{Bytes: cid, Valid: true}
		}
	}
	extID := res.ChannelConfirmationID
	var extPtr *string
	if extID != "" {
		extPtr = &extID
	}
	meta := res.RawPayload
	if len(meta) == 0 {
		meta = []byte("{}")
	}
	amountMinor := int64(res.TotalAmount * 100)
	// The caller will announce this booking: mark it pending (with a copy to
	// resend) until the PMS takes it. See domain.Delivery.
	pendingMeta := func(base []byte) ([]byte, error) {
		if !res.AwaitPMS {
			return base, nil
		}
		return domain.WithPendingDelivery(base, *res)
	}
	currency := res.Currency
	if currency == "" {
		currency = "USD"
	}

	first, last := splitName(res.GuestName)

	result := domain.SaveInserted
	err = r.pool.WithTenant(ctx, tc.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		q := pgstore.New(tx)

		// A booking this connector already sent: update it in place, and
		// only when something the PMS acts on changed. The guest row is
		// written for new bookings only; every sync used to add one.
		if connID.Valid && extPtr != nil {
			if existing, err := q.GetReservationByExternal(ctx, pgstore.GetReservationByExternalParams{
				ChannelConnectionID: connID,
				ExternalID:          extPtr,
			}); err == nil {
				res.ID = existing.ID.String()
				stored := domain.Reservation{
					Status:      existing.Status,
					CheckIn:     existing.CheckIn,
					CheckOut:    existing.CheckOut,
					TotalAmount: float64(existing.TotalAmountMinor) / 100,
					Currency:    existing.Currency,
				}
				// Never taken by the PMS: announce it again as new, whether or
				// not it changed (the PMS imports a booking once, by its id).
				undelivered := domain.DeliveryOf(existing.Metadata).State == domain.DeliveryPending
				same := stored.SameStay(domain.Reservation{
					Status: res.Status, CheckIn: res.CheckIn, CheckOut: res.CheckOut,
					TotalAmount: float64(amountMinor) / 100, Currency: currency,
				})
				switch {
				case same && !undelivered:
					result = domain.SaveUnchanged
					return nil
				case undelivered:
					result = domain.SaveInserted
				default:
					result = domain.SaveUpdated
				}
				if meta, err = pendingMeta(existing.Metadata); err != nil {
					return err
				}
				_, err = q.UpdateReservation(ctx, pgstore.UpdateReservationParams{
					ID:               existing.ID,
					Status:           res.Status,
					CheckIn:          res.CheckIn,
					CheckOut:         res.CheckOut,
					Adults:           existing.Adults,
					Children:         existing.Children,
					Currency:         currency,
					TotalAmountMinor: amountMinor,
					Metadata:         meta,
				})
				return err
			} else if !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
		}

		var err error
		if meta, err = pendingMeta(meta); err != nil {
			return err
		}
		guestID := uuid.New()
		if _, err := q.InsertGuest(ctx, pgstore.InsertGuestParams{
			ID:        guestID,
			OrgID:     orgID,
			FirstName: strPtr(first),
			LastName:  strPtr(last),
		}); err != nil {
			return err
		}

		_, err = q.InsertReservation(ctx, pgstore.InsertReservationParams{
			ID:                  resID,
			OrgID:               orgID,
			ChannelConnectionID: connID,
			PropertyID:          propID,
			ExternalID:          extPtr,
			ConfirmationCode:    extPtr,
			PrimaryGuestID:      pgtype.UUID{Bytes: guestID, Valid: true},
			Status:              res.Status,
			CheckIn:             res.CheckIn,
			CheckOut:            res.CheckOut,
			Adults:              1,
			Children:            0,
			Currency:            currency,
			TotalAmountMinor:    amountMinor,
			Metadata:            meta,
		})
		return err
	})
	return result, err
}

// MarkDelivery records whether the PMS took the booking. Delivered drops the
// copy kept for resending; pending keeps it and the error.
func (r *Repository) MarkDelivery(ctx context.Context, id string, delivered bool, lastError string) error {
	tc, err := platformauth.FromContext(ctx)
	if err != nil {
		return err
	}
	rid, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("reservations: invalid id: %w", err)
	}
	return r.pool.WithTenant(ctx, tc.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
UPDATE reservations.reservations
   SET metadata = jsonb_set(
         COALESCE(metadata, '{}'::jsonb),
         '{pms_delivery}',
         CASE WHEN $2::boolean
              THEN jsonb_build_object('state', 'delivered',
                     'attempts', COALESCE((metadata #>> '{pms_delivery,attempts}')::int, 0) + 1)
              ELSE COALESCE(metadata -> 'pms_delivery', '{}'::jsonb)
                   || jsonb_build_object('state', 'pending', 'last_error', $3::text,
                     'attempts', COALESCE((metadata #>> '{pms_delivery,attempts}')::int, 0) + 1)
         END,
         true),
       updated_at = now()
 WHERE id = $1`, rid, delivered, lastError)
		return err
	})
}

// ListUndelivered returns the bookings of a property the PMS has not taken
// yet, as they were announced, oldest first.
func (r *Repository) ListUndelivered(ctx context.Context, propertyID string) ([]domain.Reservation, error) {
	tc, err := platformauth.FromContext(ctx)
	if err != nil {
		return nil, err
	}
	pid, err := uuid.Parse(propertyID)
	if err != nil {
		return nil, fmt.Errorf("reservations: invalid property_id: %w", err)
	}
	var out []domain.Reservation
	err = r.pool.WithTenant(ctx, tc.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
SELECT id, metadata
  FROM reservations.reservations
 WHERE property_id = $1
   AND metadata #>> '{pms_delivery,state}' = 'pending'
 ORDER BY created_at
 LIMIT 100`, pid)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id uuid.UUID
			var meta []byte
			if err := rows.Scan(&id, &meta); err != nil {
				return err
			}
			d := domain.DeliveryOf(meta)
			if d.Booking == nil {
				continue
			}
			b := *d.Booking
			b.ID = id.String()
			out = append(out, b)
		}
		return rows.Err()
	})
	return out, err
}

func (r *Repository) UpdateStatus(ctx context.Context, id, status string) error {
	res, err := r.GetByID(ctx, id)
	if err != nil {
		return err
	}
	res.Status = status
	_, err = r.Save(ctx, res)
	return err
}

func mapGetRow(row pgstore.GetReservationRow) domain.Reservation {
	return mapFields(
		row.ID, row.PropertyID, row.ChannelConnectionID, row.ExternalID,
		row.CheckIn, row.CheckOut, row.Status, row.Currency, row.TotalAmountMinor, row.Metadata,
	)
}

func mapListRow(row pgstore.ListReservationsByPropertyRow) domain.Reservation {
	return mapFields(
		row.ID, row.PropertyID, row.ChannelConnectionID, row.ExternalID,
		row.CheckIn, row.CheckOut, row.Status, row.Currency, row.TotalAmountMinor, row.Metadata,
	)
}

func mapFields(
	id, propertyID uuid.UUID,
	channelConnectionID pgtype.UUID,
	externalID *string,
	checkIn, checkOut time.Time,
	status, currency string,
	totalMinor int64,
	metadata []byte,
) domain.Reservation {
	res := domain.Reservation{
		ID:          id.String(),
		PropertyID:  propertyID.String(),
		CheckIn:     checkIn,
		CheckOut:    checkOut,
		Status:      status,
		Currency:    currency,
		TotalAmount: float64(totalMinor) / 100,
		RawPayload:  metadata,
	}
	if channelConnectionID.Valid {
		res.ChannelID = uuid.UUID(channelConnectionID.Bytes).String()
	}
	if externalID != nil {
		res.ChannelConfirmationID = *externalID
	}
	return res
}

func splitName(name string) (string, string) {
	parts := strings.Fields(strings.TrimSpace(name))
	if len(parts) == 0 {
		return "", ""
	}
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], strings.Join(parts[1:], " ")
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
