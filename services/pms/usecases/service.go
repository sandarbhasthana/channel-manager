package usecases

import (
	"github.com/channel-manager/channel-manager/services/pms/ports"
)

// PmsService orchestrates PMS adapter operations.
type PmsService struct {
	adapters map[string]ports.PmsAdapter
}

// NewPmsService creates a new PmsService.
func NewPmsService() *PmsService {
	return &PmsService{
		adapters: make(map[string]ports.PmsAdapter),
	}
}

// RegisterAdapter registers a PMS adapter.
func (s *PmsService) RegisterAdapter(adapter ports.PmsAdapter) {
	s.adapters[adapter.PmsID()] = adapter
}
