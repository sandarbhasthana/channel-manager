// Package audit adapts the shared audit service to the storefront's
// AuditRecorder port.
package audit

import (
	"context"

	auditdomain "github.com/channel-manager/channel-manager/services/audit/domain"
	auditusecases "github.com/channel-manager/channel-manager/services/audit/usecases"
	"github.com/channel-manager/channel-manager/services/storefront/ports"
)

// Recorder implements ports.AuditRecorder on top of the audit service.
//
// Every storefront caller authenticates with an org-scoped integration API key
// rather than a user session, so entries are always written with actor_type
// "integration" and a null actor_id.
type Recorder struct {
	svc *auditusecases.AuditService
}

// NewRecorder wraps the audit service.
func NewRecorder(svc *auditusecases.AuditService) *Recorder {
	return &Recorder{svc: svc}
}

// Record appends the event, logging rather than returning any failure.
func (r *Recorder) Record(ctx context.Context, e ports.AuditEvent) {
	r.svc.RecordAsync(
		ctx,
		e.OrgID,
		auditdomain.ActorIntegration,
		"", // no user actor on the storefront ingress
		e.Action,
		e.ResourceType,
		e.ResourceID,
		e.Metadata,
	)
}

var _ ports.AuditRecorder = (*Recorder)(nil)
