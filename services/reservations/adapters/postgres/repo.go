package postgres

import (
	"context"
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

func (r *Repository) Save(ctx context.Context, res *domain.Reservation) error {
	tc, err := platformauth.FromContext(ctx)
	if err != nil {
		return err
	}
	if res.ID == "" {
		res.ID = uuid.NewString()
	}
	orgID, _ := uuid.Parse(tc.OrgID)
	propID, err := uuid.Parse(res.PropertyID)
	if err != nil {
		return fmt.Errorf("reservations: invalid property_id: %w", err)
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
	currency := res.Currency
	if currency == "" {
		currency = "USD"
	}

	first, last := splitName(res.GuestName)
	guestID := uuid.New()

	return r.pool.WithTenant(ctx, tc.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		q := pgstore.New(tx)
		_, err := q.InsertGuest(ctx, pgstore.InsertGuestParams{
			ID:        guestID,
			OrgID:     orgID,
			FirstName: strPtr(first),
			LastName:  strPtr(last),
		})
		if err != nil {
			return err
		}

		if connID.Valid && extPtr != nil {
			if existing, err := q.GetReservationByExternal(ctx, pgstore.GetReservationByExternalParams{
				ChannelConnectionID: connID,
				ExternalID:          extPtr,
			}); err == nil {
				_, err = q.UpdateReservation(ctx, pgstore.UpdateReservationParams{
					ID:               existing.ID,
					Status:           res.Status,
					CheckIn:          res.CheckIn,
					CheckOut:         res.CheckOut,
					Adults:           1,
					Children:         0,
					Currency:         currency,
					TotalAmountMinor: amountMinor,
					Metadata:         meta,
				})
				return err
			}
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
}

func (r *Repository) UpdateStatus(ctx context.Context, id, status string) error {
	res, err := r.GetByID(ctx, id)
	if err != nil {
		return err
	}
	res.Status = status
	return r.Save(ctx, res)
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
