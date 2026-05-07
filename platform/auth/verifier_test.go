package auth_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	auth "github.com/channel-manager/channel-manager/platform/auth"
)

// jwkKey is a minimal JWK entry for RSA public keys.
type jwkKey struct {
	Kty string `json:"kty"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	Kid string `json:"kid"`
	N   string `json:"n"`
	E   string `json:"e"`
}

type jwkSet struct {
	Keys []jwkKey `json:"keys"`
}

// generateRSAKey generates a 2048-bit RSA key; fails the test on error.
func generateRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey: %v", err)
	}
	return k
}

// newJWKSServer starts an httptest.Server that serves pub as a JWKS document.
// The server is closed automatically via t.Cleanup.
func newJWKSServer(t *testing.T, kid string, pub *rsa.PublicKey) *httptest.Server {
	t.Helper()
	set := jwkSet{Keys: []jwkKey{{
		Kty: "RSA",
		Use: "sig",
		Alg: "RS256",
		Kid: kid,
		N:   base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
		E:   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
	}}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(set)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// signToken signs an AccessTokenClaims with key using RS256 and the given kid.
func signToken(t *testing.T, key *rsa.PrivateKey, claims auth.AccessTokenClaims, kid string) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = kid
	raw, err := tok.SignedString(key)
	if err != nil {
		t.Fatalf("SignedString: %v", err)
	}
	return raw
}

func TestNewVerifier_MissingURL(t *testing.T) {
	_, err := auth.NewVerifier(context.Background(), auth.VerifierConfig{})
	if err == nil {
		t.Fatal("expected error for empty JWKSURL, got nil")
	}
}

func TestVerify(t *testing.T) {
	const kid = "test-key-1"
	const iss = "https://api.workos.com"
	const aud = "client_test"

	key := generateRSAKey(t)
	srv := newJWKSServer(t, kid, &key.PublicKey)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)

	v, err := auth.NewVerifier(ctx, auth.VerifierConfig{
		JWKSURL:  srv.URL,
		Issuer:   iss,
		Audience: aud,
	})
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}

	validClaims := func() auth.AccessTokenClaims {
		return auth.AccessTokenClaims{
			SessionID:      "sess-abc",
			OrganizationID: "org_workos123",
			Role:           "admin",
			RegisteredClaims: jwt.RegisteredClaims{
				Issuer:    iss,
				Audience:  []string{aud},
				Subject:   "user_xyz",
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
				IssuedAt:  jwt.NewNumericDate(time.Now()),
			},
		}
	}

	t.Run("valid_token", func(t *testing.T) {
		raw := signToken(t, key, validClaims(), kid)
		got, err := v.Verify(ctx, raw)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Subject() != "user_xyz" {
			t.Errorf("subject = %q, want %q", got.Subject(), "user_xyz")
		}
		if got.OrganizationID != "org_workos123" {
			t.Errorf("org_id = %q, want %q", got.OrganizationID, "org_workos123")
		}
		if got.Role != "admin" {
			t.Errorf("role = %q, want %q", got.Role, "admin")
		}
	})

	t.Run("bearer_prefix_stripped", func(t *testing.T) {
		raw := "Bearer " + signToken(t, key, validClaims(), kid)
		if _, err := v.Verify(ctx, raw); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("empty_token", func(t *testing.T) {
		_, err := v.Verify(ctx, "")
		if !errors.Is(err, auth.ErrInvalidToken) {
			t.Errorf("got %v, want ErrInvalidToken", err)
		}
	})

	t.Run("expired_token", func(t *testing.T) {
		c := validClaims()
		c.RegisteredClaims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Hour))
		raw := signToken(t, key, c, kid)
		_, err := v.Verify(ctx, raw)
		if !errors.Is(err, auth.ErrInvalidToken) {
			t.Errorf("got %v, want ErrInvalidToken", err)
		}
	})

	t.Run("wrong_issuer", func(t *testing.T) {
		c := validClaims()
		c.RegisteredClaims.Issuer = "https://evil.example.com"
		raw := signToken(t, key, c, kid)
		_, err := v.Verify(ctx, raw)
		if !errors.Is(err, auth.ErrInvalidToken) {
			t.Errorf("got %v, want ErrInvalidToken", err)
		}
	})

	t.Run("wrong_audience", func(t *testing.T) {
		c := validClaims()
		c.RegisteredClaims.Audience = []string{"wrong-audience"}
		raw := signToken(t, key, c, kid)
		_, err := v.Verify(ctx, raw)
		if !errors.Is(err, auth.ErrInvalidToken) {
			t.Errorf("got %v, want ErrInvalidToken", err)
		}
	})

	t.Run("wrong_signing_key", func(t *testing.T) {
		otherKey := generateRSAKey(t)
		// kid matches JWKS entry but token is signed with a different private key
		raw := signToken(t, otherKey, validClaims(), kid)
		_, err := v.Verify(ctx, raw)
		if !errors.Is(err, auth.ErrInvalidToken) {
			t.Errorf("got %v, want ErrInvalidToken", err)
		}
	})
}
