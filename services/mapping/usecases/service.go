package usecases

import (
	"github.com/channel-manager/channel-manager/services/mapping/ports"
)

// MappingService orchestrates room mapping operations.
type MappingService struct {
	repo ports.MappingRepository
}

// NewMappingService creates a new MappingService.
func NewMappingService(repo ports.MappingRepository) *MappingService {
	return &MappingService{
		repo: repo,
	}
}
