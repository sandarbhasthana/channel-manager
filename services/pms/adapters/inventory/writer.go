// Package inventory bridges PMS ingestion into the inventory service.
package inventory

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	invdomain "github.com/channel-manager/channel-manager/services/inventory/domain"
	invusecases "github.com/channel-manager/channel-manager/services/inventory/usecases"
	"github.com/channel-manager/channel-manager/services/pms/ports"
)

// Writer implements ports.InventoryWriter using the inventory service.
type Writer struct {
	svc *invusecases.InventoryService
}

// NewWriter creates a writer backed by the inventory service.
func NewWriter(svc *invusecases.InventoryService) *Writer {
	return &Writer{svc: svc}
}

func (w *Writer) BulkUpsertFromPMS(ctx context.Context, orgID string, inputs []ports.InventoryDayInput) (int32, string, error) {
	if len(inputs) == 0 {
		return 0, "", nil
	}
	days := make([]invdomain.InventoryDay, 0, len(inputs))
	for _, in := range inputs {
		days = append(days, invdomain.InventoryDay{
			OrgID:      orgID,
			RoomTypeID: in.RoomTypeID,
			StayDate:   in.StayDate,
			Available:  in.Available,
			StopSell:   in.StopSell,
		})
	}
	result, err := w.svc.BulkUpsertInventory(ctx, invusecases.BulkUpsertInput{
		Days:           days,
		IdempotencyKey: uuid.NewString(),
	})
	if err != nil {
		return 0, "", fmt.Errorf("pms/inventory writer: %w", err)
	}
	return result.RowsAffected, result.EventID, nil
}
