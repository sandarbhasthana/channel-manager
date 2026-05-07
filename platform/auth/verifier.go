package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
)

// VerifierConfig configures the JWT verifier. The verifier is stateless
// at the call site but holds a refresh goroutine inside the Keyfunc, so
// callers should construct a single Verifier and reuse it.
type VerifierConfig struct {
	// JWKSURL is the IdP's JSON Web Key Set endpoint. WorkOS exposes
	// this at https://api.workos.com/sso/jwks/<client_id>.
	JWKSURL string
	// Issuer is the expected `iss` claim. Empty disables the check
	// (not recommended outside tests).
	Issuer string
	// Audience is the expected `aud` value. Empty disables the check.
	// For WorkOS AuthKit access tokens this is the WorkOS client id.
	Audience string
}

// Verifier validates and parses bearer tokens against a remote JWKS.
// The underlying Keyfunc transparently refreshes the key set, so
// rotated signing keys are picked up without restarts.
type Verifier struct {
	cfg     VerifierConfig
	keyfunc keyfunc.Keyfunc
}

// ErrInvalidToken is returned for any failure during parsing,
// signature verification, or claim validation. The error wraps the
// underlying cause so it can be inspected with errors.Is/As.
var ErrInvalidToken = errors.New("auth: invalid token")

// NewVerifier constructs a Verifier and starts the JWKS refresh
// goroutine. It returns an error if the initial JWKS fetch fails.
func NewVerifier(ctx context.Context, cfg VerifierConfig) (*Verifier, error) {
	if cfg.JWKSURL == "" {
		return nil, fmt.Errorf("auth: VerifierConfig.JWKSURL is required")
	}
	kf, err := keyfunc.NewDefaultCtx(ctx, []string{cfg.JWKSURL})
	if err != nil {
		return nil, fmt.Errorf("auth: load JWKS: %w", err)
	}
	return &Verifier{cfg: cfg, keyfunc: kf}, nil
}

// Verify parses and validates a bearer token. The "Bearer " prefix is
// stripped if present; whitespace is trimmed. Successful return
// guarantees the token's signature matched a current JWKS key, the
// `exp`/`nbf`/`iat` claims are within tolerance, and the configured
// issuer/audience match.
func (v *Verifier) Verify(ctx context.Context, raw string) (*AccessTokenClaims, error) {
	tokenStr := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(raw), "Bearer "))
	if tokenStr == "" {
		return nil, fmt.Errorf("%w: empty token", ErrInvalidToken)
	}

	opts := []jwt.ParserOption{
		jwt.WithValidMethods([]string{"RS256", "RS384", "RS512"}),
	}
	if v.cfg.Issuer != "" {
		opts = append(opts, jwt.WithIssuer(v.cfg.Issuer))
	}
	if v.cfg.Audience != "" {
		opts = append(opts, jwt.WithAudience(v.cfg.Audience))
	}

	claims := &AccessTokenClaims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, v.keyfunc.KeyfuncCtx(ctx), opts...)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidToken, err)
	}
	if !token.Valid {
		return nil, fmt.Errorf("%w: token not valid", ErrInvalidToken)
	}
	return claims, nil
}
