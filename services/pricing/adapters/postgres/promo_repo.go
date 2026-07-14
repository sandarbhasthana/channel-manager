package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	platformauth "github.com/channel-manager/channel-manager/platform/auth"
	platformdb "github.com/channel-manager/channel-manager/platform/db"
	"github.com/channel-manager/channel-manager/services/pricing/domain"
	"github.com/channel-manager/channel-manager/services/pricing/ports"
)

// PromoRepository implements ports.PromoRepository against pricing.promo_codes.
type PromoRepository struct {
	pool *platformdb.Pool
}

// NewPromoRepository creates a promo repository backed by pool.
func NewPromoRepository(pool *platformdb.Pool) *PromoRepository {
	return &PromoRepository{pool: pool}
}

var _ ports.PromoRepository = (*PromoRepository)(nil)

const promoCols = `id, org_id, property_id, code, description, discount_pct,
	max_uses, uses, valid_from, valid_until, is_active, created_at, updated_at`

// orgID pulls the tenant from context. Every query below runs inside
// WithTenant, so RLS already confines rows to this org; org_id is still needed
// on insert.
func orgID(ctx context.Context) (string, error) {
	tc, err := platformauth.FromContext(ctx)
	if err != nil {
		return "", err
	}
	return tc.OrgID, nil
}

func scanPromo(row pgx.Row) (domain.PromoCode, error) {
	var (
		p          domain.PromoCode
		propertyID *string
		desc       *string
		maxUses    *int
		validFrom  *time.Time
		validUntil *time.Time
	)
	err := row.Scan(
		&p.ID, &p.OrgID, &propertyID, &p.Code, &desc, &p.DiscountPct,
		&maxUses, &p.Uses, &validFrom, &validUntil, &p.IsActive,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return domain.PromoCode{}, err
	}
	if propertyID != nil {
		p.PropertyID = *propertyID
	}
	if desc != nil {
		p.Description = *desc
	}
	p.MaxUses = maxUses
	p.ValidFrom = validFrom
	p.ValidUntil = validUntil
	return p, nil
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func (r *PromoRepository) Create(ctx context.Context, p domain.PromoCode) (domain.PromoCode, error) {
	org, err := orgID(ctx)
	if err != nil {
		return domain.PromoCode{}, err
	}

	var out domain.PromoCode
	err = r.pool.WithTenant(ctx, org, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			INSERT INTO pricing.promo_codes
				(org_id, property_id, code, description, discount_pct,
				 max_uses, valid_from, valid_until, is_active)
			VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9)
			RETURNING `+promoCols,
			org, nilIfEmpty(p.PropertyID), p.Code, nilIfEmpty(p.Description),
			p.DiscountPct, p.MaxUses, p.ValidFrom, p.ValidUntil, p.IsActive,
		)
		out, err = scanPromo(row)
		return err
	})
	if err != nil {
		return domain.PromoCode{}, fmt.Errorf("pricing: create promo: %w", err)
	}
	return out, nil
}

func (r *PromoRepository) GetByCode(ctx context.Context, code string) (domain.PromoCode, error) {
	return r.getBy(ctx, `WHERE code = $1`, code)
}

func (r *PromoRepository) GetByID(ctx context.Context, id string) (domain.PromoCode, error) {
	return r.getBy(ctx, `WHERE id = $1::uuid`, id)
}

// getBy appends an explicit org_id predicate to where. RLS already confines
// rows to the tenant; this is defence in depth, so that a connection whose role
// is exempt from RLS (a superuser, or one with BYPASSRLS) degrades to "no rows"
// rather than "every tenant's rows".
func (r *PromoRepository) getBy(ctx context.Context, where string, arg any) (domain.PromoCode, error) {
	org, err := orgID(ctx)
	if err != nil {
		return domain.PromoCode{}, err
	}

	var out domain.PromoCode
	err = r.pool.WithTenant(ctx, org, func(ctx context.Context, tx pgx.Tx) error {
		out, err = scanPromo(tx.QueryRow(ctx,
			`SELECT `+promoCols+` FROM pricing.promo_codes `+where+` AND org_id = $2::uuid`, arg, org))
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.PromoCode{}, domain.ErrPromoNotFound
	}
	if err != nil {
		return domain.PromoCode{}, fmt.Errorf("pricing: get promo: %w", err)
	}
	return out, nil
}

func (r *PromoRepository) ListByOrg(ctx context.Context) ([]domain.PromoCode, error) {
	org, err := orgID(ctx)
	if err != nil {
		return nil, err
	}

	var out []domain.PromoCode
	err = r.pool.WithTenant(ctx, org, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT `+promoCols+` FROM pricing.promo_codes ORDER BY created_at DESC`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			p, err := scanPromo(rows)
			if err != nil {
				return err
			}
			out = append(out, p)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, fmt.Errorf("pricing: list promos: %w", err)
	}
	return out, nil
}

// Update rewrites the mutable fields. `uses` is deliberately not among them:
// the counter moves only through Redeem and ReleaseRedemption.
func (r *PromoRepository) Update(ctx context.Context, p domain.PromoCode) (domain.PromoCode, error) {
	org, err := orgID(ctx)
	if err != nil {
		return domain.PromoCode{}, err
	}

	var out domain.PromoCode
	err = r.pool.WithTenant(ctx, org, func(ctx context.Context, tx pgx.Tx) error {
		out, err = scanPromo(tx.QueryRow(ctx, `
			UPDATE pricing.promo_codes SET
				property_id  = $2::uuid,
				description  = $3,
				discount_pct = $4,
				max_uses     = $5,
				valid_from   = $6,
				valid_until  = $7,
				is_active    = $8
			WHERE id = $1::uuid
			RETURNING `+promoCols,
			p.ID, nilIfEmpty(p.PropertyID), nilIfEmpty(p.Description),
			p.DiscountPct, p.MaxUses, p.ValidFrom, p.ValidUntil, p.IsActive,
		))
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.PromoCode{}, domain.ErrPromoNotFound
	}
	if err != nil {
		return domain.PromoCode{}, fmt.Errorf("pricing: update promo: %w", err)
	}
	return out, nil
}

func (r *PromoRepository) Delete(ctx context.Context, id string) error {
	org, err := orgID(ctx)
	if err != nil {
		return err
	}
	err = r.pool.WithTenant(ctx, org, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `DELETE FROM pricing.promo_codes WHERE id = $1::uuid`, id)
		return err
	})
	if err != nil {
		return fmt.Errorf("pricing: delete promo: %w", err)
	}
	return nil
}

// Redeem increments `uses` in a single conditional UPDATE.
//
// Every rule is in the WHERE clause, so the check and the increment cannot be
// separated by another transaction. If no row matches, the code is either
// absent or ineligible — a second, unconditional read then distinguishes which,
// purely to produce a useful error. That read is outside the guarantee and may
// race, but only affects the error message, never whether a redemption occurred.
//
// The org_id predicate is redundant under RLS and deliberately so: without it,
// a role exempt from RLS would match this code string in *every* tenant and
// burn all their counters in one statement.
func (r *PromoRepository) Redeem(ctx context.Context, code, propertyID string, at time.Time) (domain.PromoCode, error) {
	org, err := orgID(ctx)
	if err != nil {
		return domain.PromoCode{}, err
	}

	var out domain.PromoCode
	err = r.pool.WithTenant(ctx, org, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			UPDATE pricing.promo_codes SET uses = uses + 1
			WHERE code = $1
			  AND org_id = $4::uuid
			  AND is_active
			  AND (property_id IS NULL OR property_id = $2::uuid)
			  AND (valid_from  IS NULL OR valid_from  <= $3)
			  AND (valid_until IS NULL OR valid_until >  $3)
			  AND (max_uses    IS NULL OR uses < max_uses)
			RETURNING `+promoCols,
			code, nilIfEmpty(propertyID), at, org,
		)
		out, err = scanPromo(row)
		return err
	})

	if errors.Is(err, pgx.ErrNoRows) {
		// Nothing was redeemed. Work out why, for the guest's benefit.
		existing, getErr := r.GetByCode(ctx, code)
		if getErr != nil {
			return domain.PromoCode{}, getErr // ErrPromoNotFound, usually
		}
		if reason := existing.Validate(at, propertyID); reason != nil {
			return domain.PromoCode{}, reason
		}
		// Rules pass now but the UPDATE matched nothing: another transaction
		// consumed the last redemption between the two statements.
		return domain.PromoCode{}, domain.ErrPromoExhausted
	}
	if err != nil {
		return domain.PromoCode{}, fmt.Errorf("pricing: redeem promo: %w", err)
	}
	return out, nil
}

// ReleaseRedemption gives a redemption back. Clamped at zero so a double
// release cannot manufacture capacity.
func (r *PromoRepository) ReleaseRedemption(ctx context.Context, code string) error {
	org, err := orgID(ctx)
	if err != nil {
		return err
	}
	err = r.pool.WithTenant(ctx, org, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`UPDATE pricing.promo_codes SET uses = uses - 1
			 WHERE code = $1 AND org_id = $2::uuid AND uses > 0`, code, org)
		return err
	})
	if err != nil {
		return fmt.Errorf("pricing: release promo redemption: %w", err)
	}
	return nil
}
