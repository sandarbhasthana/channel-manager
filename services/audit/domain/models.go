package domain

import (
	"encoding/json"
	"time"
)

// ActorType classifies who performed an audited action. Mirrors the
// audit.audit_events.actor_type CHECK constraint.
type ActorType string

const (
	// ActorUser is a human acting through a dashboard session.
	ActorUser ActorType = "user"
	// ActorSystem is an internal job with no human initiator.
	ActorSystem ActorType = "system"
	// ActorIntegration is a machine caller authenticated by an API key,
	// such as a booking engine on the storefront ingress.
	ActorIntegration ActorType = "integration"
)

// AuditEntry is a single append-only record in audit.audit_events.
//
// The table is effectively WORM: policy grants INSERT and SELECT only, so an
// entry is never updated or deleted once written.
type AuditEntry struct {
	ID    string `json:"id"`
	OrgID string `json:"org_id"`

	// ActorID is the IdP subject for a user, or empty for system actors.
	ActorID   string    `json:"actor_id,omitempty"`
	ActorType ActorType `json:"actor_type"`

	// Action is a dotted verb such as "reservation.create" or "inventory.set".
	Action       string `json:"action"`
	ResourceType string `json:"resource_type"`
	ResourceID   string `json:"resource_id"`

	// RequestID correlates with platform/http; TraceID with OpenTelemetry.
	RequestID string `json:"request_id,omitempty"`
	TraceID   string `json:"trace_id,omitempty"`

	// Before and After capture the mutation. Both are optional: a create has
	// no Before, a read-only audited action has neither.
	Before json.RawMessage `json:"before,omitempty"`
	After  json.RawMessage `json:"after,omitempty"`

	// Metadata carries action-specific context. Never nil on write; the
	// repository substitutes an empty object.
	Metadata json.RawMessage `json:"metadata,omitempty"`

	CreatedAt time.Time `json:"created_at"`
}
