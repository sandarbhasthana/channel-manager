package ports

import (
	"context"

	"github.com/channel-manager/channel-manager/services/mapping/domain"
)

// MappingRepository provides persistence for room mappings.
type MappingRepository interface {
	GetByID(ctx context.Context, id string) (*domain.RoomMapping, error)
	ListByProperty(ctx context.Context, orgID, channelID string) ([]domain.RoomMapping, error)
	FindByInternal(ctx context.Context, internalRoomTypeID, channelID string) (*domain.RoomMapping, error)
	FindByExternal(ctx context.Context, externalID, channelID string) (*domain.RoomMapping, error)
	Save(ctx context.Context, mapping *domain.RoomMapping) error
	Delete(ctx context.Context, id string) error
}
