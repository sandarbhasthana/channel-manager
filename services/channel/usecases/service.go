package usecases

import (
	"github.com/channel-manager/channel-manager/services/channel/ports"
)

// ChannelService orchestrates channel adapter operations.
type ChannelService struct {
	adapters map[string]ports.ChannelAdapter
}

// NewChannelService creates a new ChannelService.
func NewChannelService() *ChannelService {
	return &ChannelService{
		adapters: make(map[string]ports.ChannelAdapter),
	}
}

// RegisterAdapter registers a channel adapter.
func (s *ChannelService) RegisterAdapter(adapter ports.ChannelAdapter) {
	s.adapters[adapter.ChannelID()] = adapter
}
