package integration

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"

	platformauth "github.com/channel-manager/channel-manager/platform/auth"
	platformdb "github.com/channel-manager/channel-manager/platform/db"
)

const keyPrefixLabel = "cm_live"

// APIKeyRecord is a stored integration key (secret not included).
type APIKeyRecord struct {
	ID        string     `json:"id"`
	OrgID     string     `json:"org_id"`
	Name      string     `json:"name"`
	KeyPrefix string     `json:"key_prefix"`
	Scopes    []string   `json:"scopes"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// CreateKeyResult is returned once when a key is generated.
type CreateKeyResult struct {
	Record    APIKeyRecord `json:"record"`
	SecretKey string       `json:"secret_key"`
}

// KeyStore manages DB-backed integration API keys (Phase B).
type KeyStore struct {
	pool *platformdb.Pool
}

// NewKeyStore creates a keystore backed by Postgres.
func NewKeyStore(pool *platformdb.Pool) *KeyStore {
	return &KeyStore{pool: pool}
}

// CreateKey generates and stores a new API key for the org in ctx.
func (k *KeyStore) CreateKey(ctx context.Context, name string, createdBy string, expiresAt *time.Time) (CreateKeyResult, error) {
	tc, err := platformauth.FromContext(ctx)
	if err != nil {
		return CreateKeyResult{}, err
	}
	id := uuid.New()
	prefix := id.String()[:8]
	secret, err := randomSecret(32)
	if err != nil {
		return CreateKeyResult{}, err
	}
	fullKey := fmt.Sprintf("%s_%s_%s", keyPrefixLabel, prefix, secret)
	hash, err := bcrypt.GenerateFromPassword([]byte(fullKey), bcrypt.DefaultCost)
	if err != nil {
		return CreateKeyResult{}, fmt.Errorf("integration: hash key: %w", err)
	}

	var record APIKeyRecord
	err = k.pool.WithTenant(ctx, tc.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			INSERT INTO tenancy.integration_api_keys
			    (id, org_id, name, key_prefix, key_hash, created_by, expires_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			RETURNING id, org_id, name, key_prefix, scopes, expires_at, revoked_at, created_at`,
			id, tc.OrgID, name, prefix, string(hash), createdBy, expiresAt,
		)
		var idUUID uuid.UUID
		var orgUUID uuid.UUID
		if err := row.Scan(
			&idUUID, &orgUUID, &record.Name, &record.KeyPrefix, &record.Scopes,
			&record.ExpiresAt, &record.RevokedAt, &record.CreatedAt,
		); err != nil {
			return err
		}
		record.ID = idUUID.String()
		record.OrgID = orgUUID.String()
		return nil
	})
	if err != nil {
		return CreateKeyResult{}, fmt.Errorf("integration: create key: %w", err)
	}
	return CreateKeyResult{Record: record, SecretKey: fullKey}, nil
}

// ListKeys returns non-revoked keys for the org in ctx.
func (k *KeyStore) ListKeys(ctx context.Context) ([]APIKeyRecord, error) {
	tc, err := platformauth.FromContext(ctx)
	if err != nil {
		return nil, err
	}
	var out []APIKeyRecord
	err = k.pool.WithTenant(ctx, tc.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id, org_id, name, key_prefix, scopes, expires_at, revoked_at, created_at
			  FROM tenancy.integration_api_keys
			 WHERE revoked_at IS NULL
			 ORDER BY created_at DESC`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var rec APIKeyRecord
			var idUUID, orgUUID uuid.UUID
			if err := rows.Scan(
				&idUUID, &orgUUID, &rec.Name, &rec.KeyPrefix, &rec.Scopes,
				&rec.ExpiresAt, &rec.RevokedAt, &rec.CreatedAt,
			); err != nil {
				return err
			}
			rec.ID = idUUID.String()
			rec.OrgID = orgUUID.String()
			out = append(out, rec)
		}
		return rows.Err()
	})
	return out, err
}

// RevokeKey marks a key revoked for the org in ctx.
func (k *KeyStore) RevokeKey(ctx context.Context, keyID string) error {
	tc, err := platformauth.FromContext(ctx)
	if err != nil {
		return err
	}
	return k.pool.WithTenant(ctx, tc.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		ct, err := tx.Exec(ctx, `
			UPDATE tenancy.integration_api_keys
			   SET revoked_at = now(), updated_at = now()
			 WHERE id = $1 AND org_id = $2 AND revoked_at IS NULL`, keyID, tc.OrgID)
		if err != nil {
			return err
		}
		if ct.RowsAffected() == 0 {
			return pgx.ErrNoRows
		}
		return nil
	})
}

// ResolveOrgID validates a bearer token against DB keys (global lookup by prefix).
func (k *KeyStore) ResolveOrgID(ctx context.Context, token string) (string, error) {
	prefix, err := parseKeyPrefix(token)
	if err != nil {
		return "", nil
	}
	// Lookup by prefix without tenant — prefix is globally unique.
	pool := k.pool.Inner()
	var orgID, hash string
	var expiresAt, revokedAt *time.Time
	err = pool.QueryRow(ctx, `
		SELECT org_id::text, key_hash, expires_at, revoked_at
		  FROM tenancy.resolve_integration_api_key($1)
		`, prefix).Scan(&orgID, &hash, &expiresAt, &revokedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", nil
		}
		return "", err
	}
	if revokedAt != nil {
		return "", nil
	}
	if expiresAt != nil && time.Now().After(*expiresAt) {
		return "", nil
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(token)); err != nil {
		return "", nil
	}
	return orgID, nil
}

func parseKeyPrefix(token string) (string, error) {
	// cm_live_<prefix>_<secret>
	if !strings.HasPrefix(token, keyPrefixLabel+"_") {
		return "", fmt.Errorf("invalid key format")
	}
	rest := strings.TrimPrefix(token, keyPrefixLabel+"_")
	parts := strings.SplitN(rest, "_", 2)
	if len(parts) != 2 || len(parts[0]) < 8 {
		return "", fmt.Errorf("invalid key format")
	}
	return parts[0], nil
}

func randomSecret(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
