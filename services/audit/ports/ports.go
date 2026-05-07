package ports

import (
	"context"

	"github.com/channel-manager/channel-manager/services/audit/domain"
)

// AuditRepository provides persistence for audit entries.
type AuditRepository interface {
	Append(ctx context.Context, entry *domain.AuditEntry) error
	ListByResource(ctx context.Context, resourceType, resourceID string) ([]domain.AuditEntry, error)
	ListByActor(ctx context.Context, actorID string) ([]domain.AuditEntry, error)
	ListByOrg(ctx context.Context, orgID string, limit, offset int) ([]domain.AuditEntry, error)
}
