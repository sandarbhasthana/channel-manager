package connect

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	channelv1 "github.com/channel-manager/channel-manager/gen/go/channel/v1"
	"github.com/channel-manager/channel-manager/gen/go/channel/v1/channelv1connect"
	platformauth "github.com/channel-manager/channel-manager/platform/auth"
	"github.com/channel-manager/channel-manager/services/channel/domain"
	"github.com/channel-manager/channel-manager/services/channel/usecases"
)

// ChannelHandler implements channelv1connect.ChannelServiceHandler.
type ChannelHandler struct {
	channelv1connect.UnimplementedChannelServiceHandler
	svc *usecases.ChannelService
}

// NewChannelHandler creates a new ChannelHandler.
func NewChannelHandler(svc *usecases.ChannelService) *ChannelHandler {
	return &ChannelHandler{svc: svc}
}

func (h *ChannelHandler) ListChannels(
	ctx context.Context,
	req *connect.Request[channelv1.ListChannelsRequest],
) (*connect.Response[channelv1.ListChannelsResponse], error) {
	if req.Msg.GetPropertyId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("property_id is required"))
	}
	chs, err := h.svc.ListChannels(ctx, req.Msg.GetPropertyId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	pbChs := make([]*channelv1.Channel, 0, len(chs))
	for _, c := range chs {
		pbChs = append(pbChs, channelToProto(c))
	}
	return connect.NewResponse(&channelv1.ListChannelsResponse{Channels: pbChs}), nil
}

func (h *ChannelHandler) GetChannel(
	ctx context.Context,
	req *connect.Request[channelv1.GetChannelRequest],
) (*connect.Response[channelv1.GetChannelResponse], error) {
	if req.Msg.GetId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is required"))
	}
	ch, err := h.svc.GetChannel(ctx, req.Msg.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&channelv1.GetChannelResponse{Channel: channelToProto(ch)}), nil
}

func (h *ChannelHandler) ConnectChannel(
	ctx context.Context,
	req *connect.Request[channelv1.ConnectChannelRequest],
) (*connect.Response[channelv1.ConnectChannelResponse], error) {
	r := req.Msg
	if r.GetPropertyId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("property_id is required"))
	}
	if r.GetConnectionId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("connection_id is required"))
	}
	if r.GetExternalPropertyId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("external_property_id is required"))
	}

	tc, err := platformauth.FromContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	ch, err := h.svc.ConnectChannel(ctx, domain.Channel{
		OrgID:              tc.OrgID,
		PropertyID:         r.GetPropertyId(),
		ConnectionID:       r.GetConnectionId(),
		ExternalPropertyID: r.GetExternalPropertyId(),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&channelv1.ConnectChannelResponse{Channel: channelToProto(ch)}), nil
}

func (h *ChannelHandler) PauseChannel(
	ctx context.Context,
	req *connect.Request[channelv1.PauseChannelRequest],
) (*connect.Response[channelv1.PauseChannelResponse], error) {
	if req.Msg.GetId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is required"))
	}
	ch, err := h.svc.PauseChannel(ctx, req.Msg.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&channelv1.PauseChannelResponse{Channel: channelToProto(ch)}), nil
}

func (h *ChannelHandler) ResumeChannel(
	ctx context.Context,
	req *connect.Request[channelv1.ResumeChannelRequest],
) (*connect.Response[channelv1.ResumeChannelResponse], error) {
	if req.Msg.GetId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is required"))
	}
	ch, err := h.svc.ResumeChannel(ctx, req.Msg.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&channelv1.ResumeChannelResponse{Channel: channelToProto(ch)}), nil
}

func (h *ChannelHandler) DisconnectChannel(
	ctx context.Context,
	req *connect.Request[channelv1.DisconnectChannelRequest],
) (*connect.Response[channelv1.DisconnectChannelResponse], error) {
	if req.Msg.GetId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is required"))
	}
	ch, err := h.svc.DisconnectChannel(ctx, req.Msg.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&channelv1.DisconnectChannelResponse{Channel: channelToProto(ch)}), nil
}
