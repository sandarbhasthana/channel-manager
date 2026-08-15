package integration

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// x509ParsePKIXEd25519 unwraps a DER SubjectPublicKeyInfo and asserts it really
// is an Ed25519 key — x509.ParsePKIXPublicKey happily returns an RSA or ECDSA
// key, and passing one of those to the Ed25519 verifier would panic rather than
// fail the request.
func x509ParsePKIXEd25519(der []byte) (ed25519.PublicKey, error) {
	parsed, err := x509.ParsePKIXPublicKey(der)
	if err != nil {
		return nil, fmt.Errorf("parse public key: %w", err)
	}
	key, ok := parsed.(ed25519.PublicKey)
	if !ok {
		return nil, fmt.Errorf("expected an ed25519 public key, got %T", parsed)
	}
	return key, nil
}

// PmsClaims are the claims a bundled PMS asserts about a request.
//
// The organization is identified by its PMS id (`org_ext`), not by anything this
// service issued. That is the point of the bundled model: the PMS is the system
// of record for tenancy, so it names the tenant and we materialize it.
//
// The acting user travels with the request. Previously every PMS-originated call
// arrived as the synthetic actor "integration:pms", which made the audit log
// unable to answer the only question anyone asks of it — who did this.
type PmsClaims struct {
	jwt.RegisteredClaims
	OrgExternalID string `json:"org_ext"`
	OrgName       string `json:"org_name,omitempty"`
	ActorUserID   string `json:"act_sub,omitempty"`
	ActorRole     string `json:"act_role,omitempty"`
	ActorEmail    string `json:"act_email,omitempty"`
}

// ErrNoPmsToken signals that the bearer value is not a PMS assertion at all, so
// the caller should fall through to the other authentication branches rather than
// reject the request. Distinct from a token that IS a PMS assertion but fails
// verification, which must never fall through — otherwise a forged token could be
// retried as an API key lookup and the failure would be reported as the wrong
// kind of error.
var ErrNoPmsToken = errors.New("integration: not a PMS assertion")

// PmsVerifierConfig configures verification of PMS-signed assertions.
type PmsVerifierConfig struct {
	// PublicKeysJSON maps key id → Ed25519 public key, either a PEM block or raw
	// base64 of the 32-byte key. A map rather than a single key so a second key
	// can be published before the PMS starts signing with it, which is what makes
	// rotation possible without a synchronized restart of both services.
	PublicKeysJSON string
	// Issuer is the expected `iss`. Empty disables the check.
	Issuer string
	// Audience is the expected `aud`. Empty disables the check.
	Audience string
	// Leeway absorbs clock drift between the PMS and this service. Tokens live
	// ~90s, so without a tolerance two containers whose clocks differ by a few
	// seconds reject valid tokens intermittently — which surfaces as flaky 401s
	// under load rather than as an obvious misconfiguration.
	Leeway time.Duration
}

// PmsVerifier validates Ed25519 assertions minted by a bundled PMS.
type PmsVerifier struct {
	keys   map[string]ed25519.PublicKey
	parser *jwt.Parser
}

// NewPmsVerifier parses the configured public keys. Returns (nil, nil) when no
// keys are configured, so a deployment that has not adopted bundled auth keeps
// working on API keys alone.
func NewPmsVerifier(cfg PmsVerifierConfig) (*PmsVerifier, error) {
	if strings.TrimSpace(cfg.PublicKeysJSON) == "" {
		return nil, nil
	}
	raw := map[string]string{}
	if err := json.Unmarshal([]byte(cfg.PublicKeysJSON), &raw); err != nil {
		return nil, fmt.Errorf("integration: parse PMS public keys: %w", err)
	}
	if len(raw) == 0 {
		return nil, nil
	}

	keys := make(map[string]ed25519.PublicKey, len(raw))
	for kid, encoded := range raw {
		key, err := parseEd25519PublicKey(encoded)
		if err != nil {
			return nil, fmt.Errorf("integration: public key %q: %w", kid, err)
		}
		keys[kid] = key
	}

	if cfg.Leeway <= 0 {
		cfg.Leeway = 30 * time.Second
	}

	opts := []jwt.ParserOption{
		// Pinned, not negotiated. Accepting whatever the header names is how
		// algorithm-confusion attacks work.
		jwt.WithValidMethods([]string{"EdDSA"}),
		jwt.WithLeeway(cfg.Leeway),
		jwt.WithExpirationRequired(),
	}
	if cfg.Issuer != "" {
		opts = append(opts, jwt.WithIssuer(cfg.Issuer))
	}
	if cfg.Audience != "" {
		opts = append(opts, jwt.WithAudience(cfg.Audience))
	}

	return &PmsVerifier{keys: keys, parser: jwt.NewParser(opts...)}, nil
}

// Verify parses and validates a PMS assertion.
//
// Returns ErrNoPmsToken when the value is not a JWT this verifier should handle,
// so the caller can try the API-key branches. Any other error means the token
// claimed to be a PMS assertion and was not valid.
func (v *PmsVerifier) Verify(token string) (*PmsClaims, error) {
	if v == nil {
		return nil, ErrNoPmsToken
	}
	// A JWT has two dots; an API key (`cm_live_<prefix>_<secret>`) has none. This
	// cheap shape test is what lets an invalid signature be a hard failure rather
	// than a fall-through.
	if strings.Count(token, ".") != 2 {
		return nil, ErrNoPmsToken
	}

	claims := &PmsClaims{}
	_, err := v.parser.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
		kid, _ := t.Header["kid"].(string)
		if kid == "" {
			return nil, errors.New("missing kid header")
		}
		key, ok := v.keys[kid]
		if !ok {
			return nil, fmt.Errorf("unknown kid %q", kid)
		}
		return key, nil
	})
	if err != nil {
		return nil, fmt.Errorf("integration: invalid PMS assertion: %w", err)
	}
	if claims.OrgExternalID == "" {
		return nil, errors.New("integration: PMS assertion has no org_ext claim")
	}
	return claims, nil
}

// parseEd25519PublicKey accepts a PEM-encoded SubjectPublicKeyInfo block or a
// bare base64 32-byte key. Both because PEM is what key-generation tools emit and
// what survives a copy-paste into an env var intact, while the bare form is
// easier to hand-write in a test.
func parseEd25519PublicKey(encoded string) (ed25519.PublicKey, error) {
	trimmed := strings.TrimSpace(encoded)
	if trimmed == "" {
		return nil, errors.New("empty key")
	}

	if strings.Contains(trimmed, "-----BEGIN") {
		// Env vars commonly carry literal "\n" rather than real newlines.
		normalized := strings.ReplaceAll(trimmed, "\\n", "\n")
		block, _ := pem.Decode([]byte(normalized))
		if block == nil {
			return nil, errors.New("malformed PEM block")
		}
		key, err := x509ParsePKIXEd25519(block.Bytes)
		if err != nil {
			return nil, err
		}
		return key, nil
	}

	for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		if decoded, err := enc.DecodeString(trimmed); err == nil {
			if len(decoded) == ed25519.PublicKeySize {
				return ed25519.PublicKey(decoded), nil
			}
			// Could still be a DER SubjectPublicKeyInfo that was base64'd without
			// the PEM armour.
			if key, err := x509ParsePKIXEd25519(decoded); err == nil {
				return key, nil
			}
		}
	}
	return nil, fmt.Errorf("expected PEM or base64 ed25519 key (%d bytes)", ed25519.PublicKeySize)
}
