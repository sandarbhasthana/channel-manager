package connect

import (
	"google.golang.org/protobuf/types/known/timestamppb"

	channelv1 "github.com/channel-manager/channel-manager/gen/go/channel/v1"
	"github.com/channel-manager/channel-manager/services/channel/domain"
)

// providerToChannelKind maps a provider string to the proto ChannelKind enum.
func providerToChannelKind(provider string) channelv1.ChannelKind {
	switch provider {
	case "airbnb":
		return channelv1.ChannelKind_CHANNEL_KIND_AIRBNB
	case "bookingcom":
		return channelv1.ChannelKind_CHANNEL_KIND_BOOKING_COM
	case "expedia":
		return channelv1.ChannelKind_CHANNEL_KIND_EXPEDIA
	case "agoda":
		return channelv1.ChannelKind_CHANNEL_KIND_AGODA
	case "direct":
		return channelv1.ChannelKind_CHANNEL_KIND_DIRECT
	default:
		return channelv1.ChannelKind_CHANNEL_KIND_UNSPECIFIED
	}
}

// channelKindToProvider maps a proto ChannelKind to a provider string.
func channelKindToProvider(kind channelv1.ChannelKind) string {
	switch kind {
	case channelv1.ChannelKind_CHANNEL_KIND_AIRBNB:
		return "airbnb"
	case channelv1.ChannelKind_CHANNEL_KIND_BOOKING_COM:
		return "bookingcom"
	case channelv1.ChannelKind_CHANNEL_KIND_EXPEDIA:
		return "expedia"
	case channelv1.ChannelKind_CHANNEL_KIND_AGODA:
		return "agoda"
	case channelv1.ChannelKind_CHANNEL_KIND_DIRECT:
		return "direct"
	default:
		return ""
	}
}

// statusToChannelStatus maps a domain status string to proto ChannelStatus.
func statusToChannelStatus(status string) channelv1.ChannelStatus {
	switch status {
	case "active":
		return channelv1.ChannelStatus_CHANNEL_STATUS_ACTIVE
	case "paused":
		return channelv1.ChannelStatus_CHANNEL_STATUS_PAUSED
	case "disconnected":
		return channelv1.ChannelStatus_CHANNEL_STATUS_DISCONNECTED
	case "error":
		return channelv1.ChannelStatus_CHANNEL_STATUS_ERROR
	default:
		return channelv1.ChannelStatus_CHANNEL_STATUS_UNSPECIFIED
	}
}

// statusToConnectionStatus maps a domain status string to proto ConnectionStatus.
func statusToConnectionStatus(status string) channelv1.ConnectionStatus {
	switch status {
	case "inactive":
		return channelv1.ConnectionStatus_CONNECTION_STATUS_INACTIVE
	case "active":
		return channelv1.ConnectionStatus_CONNECTION_STATUS_ACTIVE
	case "error":
		return channelv1.ConnectionStatus_CONNECTION_STATUS_ERROR
	case "disabled":
		return channelv1.ConnectionStatus_CONNECTION_STATUS_DISABLED
	default:
		return channelv1.ConnectionStatus_CONNECTION_STATUS_UNSPECIFIED
	}
}

// connectionStatusToDomain maps a proto ConnectionStatus to a domain status string.
// Returns "" for UNSPECIFIED so the service treats it as "do not update".
func connectionStatusToDomain(s channelv1.ConnectionStatus) string {
	switch s {
	case channelv1.ConnectionStatus_CONNECTION_STATUS_ACTIVE:
		return "active"
	case channelv1.ConnectionStatus_CONNECTION_STATUS_INACTIVE:
		return "inactive"
	case channelv1.ConnectionStatus_CONNECTION_STATUS_DISABLED:
		return "disabled"
	case channelv1.ConnectionStatus_CONNECTION_STATUS_ERROR:
		return "error"
	default:
		return ""
	}
}

// connectionToProto maps a domain Connection to a proto Connection.
func connectionToProto(c domain.Connection) *channelv1.Connection {
	pb := &channelv1.Connection{
		Id:        c.ID,
		Kind:      providerToChannelKind(c.Provider),
		Name:      c.Name,
		Status:    statusToConnectionStatus(c.Status),
		CreatedAt: timestamppb.New(c.CreatedAt),
		UpdatedAt: timestamppb.New(c.UpdatedAt),
	}
	return pb
}

// channelToProto maps a domain Channel to a proto Channel.
func channelToProto(c domain.Channel) *channelv1.Channel {
	pb := &channelv1.Channel{
		Id:                 c.ID,
		PropertyId:         c.PropertyID,
		ConnectionId:       c.ConnectionID,
		Kind:               providerToChannelKind(c.Provider),
		Status:             statusToChannelStatus(c.Status),
		ExternalPropertyId: c.ExternalPropertyID,
		LastError:          c.LastError,
		CreatedAt:          timestamppb.New(c.CreatedAt),
		UpdatedAt:          timestamppb.New(c.UpdatedAt),
	}
	if c.LastSyncAt != nil {
		pb.LastSyncAt = timestamppb.New(*c.LastSyncAt)
	}
	return pb
}
