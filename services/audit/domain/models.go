package domain

import (
	"encoding/json"
	"time"
)

// AuditEntry represents a single auditable action in the system.
type AuditEntry struct {
	ID           string          `json:"id"`
	OrgID        string          `json:"org_id"`
	ActorID      string          `json:"actor_id"`
	Action       string          `json:"action"`
	ResourceType string          `json:"resource_type"`
	ResourceID   string          `json:"resource_id"`
	Payload      json.RawMessage `json:"payload,omitempty"`
	Timestamp    time.Time       `json:"timestamp"`
}
