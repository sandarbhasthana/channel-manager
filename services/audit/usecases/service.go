// Package usecases orchestrates audit logging.
package usecases

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/channel-manager/channel-manager/services/audit/domain"
	"github.com/channel-manager/channel-manager/services/audit/ports"
)

// AuditService writes and reads the append-only audit trail.
type AuditService struct {
	repo ports.AuditRepository
	log  *slog.Logger
}

// NewAuditService creates a new AuditService.
func NewAuditService(repo ports.AuditRepository) *AuditService {
	return &AuditService{
		repo: repo,
		log:  slog.Default().With("service", "audit"),
	}
}

// Append writes an audit entry, returning its assigned id.
func (s *AuditService) Append(ctx context.Context, entry *domain.AuditEntry) (string, error) {
	if err := s.repo.Append(ctx, entry); err != nil {
		return "", err
	}
	return entry.ID, nil
}

// Record is a convenience wrapper for the common case: an action on a resource
// with some metadata, no before/after diff.
func (s *AuditService) Record(
	ctx context.Context,
	orgID string,
	actorType domain.ActorType,
	actorID, action, resourceType, resourceID string,
	metadata map[string]any,
) (string, error) {
	var meta json.RawMessage
	if len(metadata) > 0 {
		encoded, err := json.Marshal(metadata)
		if err != nil {
			return "", err
		}
		meta = encoded
	}
	return s.Append(ctx, &domain.AuditEntry{
		OrgID:        orgID,
		ActorID:      actorID,
		ActorType:    actorType,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Metadata:     meta,
	})
}

// RecordAsync writes an audit entry without failing the caller.
//
// Audit is observability, not a precondition: a guest's booking must not fail
// because an audit insert failed. Callers on a request path use this and accept
// that a dropped entry is logged at ERROR rather than surfaced to the guest.
// Callers that need the audit write to be part of their transaction, or to
// block on failure, must use Append.
func (s *AuditService) RecordAsync(
	ctx context.Context,
	orgID string,
	actorType domain.ActorType,
	actorID, action, resourceType, resourceID string,
	metadata map[string]any,
) {
	if _, err := s.Record(ctx, orgID, actorType, actorID, action, resourceType, resourceID, metadata); err != nil {
		s.log.Error("audit append failed",
			"err", err, "action", action,
			"resource_type", resourceType, "resource_id", resourceID, "org_id", orgID)
	}
}

// ListByResource returns the audit trail for one resource, newest first.
func (s *AuditService) ListByResource(ctx context.Context, resourceType, resourceID string) ([]domain.AuditEntry, error) {
	return s.repo.ListByResource(ctx, resourceType, resourceID)
}

// ListByActor returns the audit trail for one actor, newest first.
func (s *AuditService) ListByActor(ctx context.Context, actorID string) ([]domain.AuditEntry, error) {
	return s.repo.ListByActor(ctx, actorID)
}

// ListByOrg returns a page of the org's audit trail, newest first.
func (s *AuditService) ListByOrg(ctx context.Context, orgID string, limit, offset int) ([]domain.AuditEntry, error) {
	return s.repo.ListByOrg(ctx, orgID, limit, offset)
}
