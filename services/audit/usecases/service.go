package usecases

import (
	"github.com/channel-manager/channel-manager/services/audit/ports"
)

// AuditService orchestrates audit logging operations.
type AuditService struct {
	repo ports.AuditRepository
}

// NewAuditService creates a new AuditService.
func NewAuditService(repo ports.AuditRepository) *AuditService {
	return &AuditService{
		repo: repo,
	}
}
