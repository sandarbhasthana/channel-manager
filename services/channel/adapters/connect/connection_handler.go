// Package connect implements Connect-RPC handlers for the channel service.
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

// ConnectionHandler implements channelv1connect.ConnectionServiceHandler.
type ConnectionHandler struct {
	channelv1connect.UnimplementedConnectionServiceHandler
	svc *usecases.ChannelService
}

// NewConnectionHandler creates a new ConnectionHandler.
func NewConnectionHandler(svc *usecases.ChannelService) *ConnectionHandler {
	return &ConnectionHandler{svc: svc}
}

func (h *ConnectionHandler) CreateConnection(
	ctx context.Context,
	req *connect.Request[channelv1.CreateConnectionRequest],
) (*connect.Response[channelv1.CreateConnectionResponse], error) {
	r := req.Msg
	if r.GetKind() == channelv1.ChannelKind_CHANNEL_KIND_UNSPECIFIED {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("kind is required"))
	}
	if r.GetName() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name is required"))
	}

	tc, err := platformauth.FromContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	conn, err := h.svc.CreateConnection(ctx, domain.Connection{
		OrgID:    tc.OrgID,
		Provider: channelKindToProvider(r.GetKind()),
		Name:     r.GetName(),
	}, r.GetCredentials())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&channelv1.CreateConnectionResponse{
		Connection: connectionToProto(conn),
	}), nil
}

func (h *ConnectionHandler) GetConnection(
	ctx context.Context,
	req *connect.Request[channelv1.GetConnectionRequest],
) (*connect.Response[channelv1.GetConnectionResponse], error) {
	if req.Msg.GetId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is required"))
	}
	conn, err := h.svc.GetConnection(ctx, req.Msg.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&channelv1.GetConnectionResponse{
		Connection: connectionToProto(conn),
	}), nil
}

func (h *ConnectionHandler) ListConnections(
	ctx context.Context,
	req *connect.Request[channelv1.ListConnectionsRequest],
) (*connect.Response[channelv1.ListConnectionsResponse], error) {
	tc, err := platformauth.FromContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	conns, err := h.svc.ListConnections(ctx, tc.OrgID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	pbConns := make([]*channelv1.Connection, 0, len(conns))
	for _, c := range conns {
		pbConns = append(pbConns, connectionToProto(c))
	}
	return connect.NewResponse(&channelv1.ListConnectionsResponse{
		Connections: pbConns,
	}), nil
}

func (h *ConnectionHandler) UpdateConnection(
	ctx context.Context,
	req *connect.Request[channelv1.UpdateConnectionRequest],
) (*connect.Response[channelv1.UpdateConnectionResponse], error) {
	r := req.Msg
	if r.GetId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is required"))
	}
	if err := h.svc.UpdateConnection(ctx, r.GetId(), r.GetName(), r.GetCredentials(), connectionStatusToDomain(r.GetStatus())); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	conn, err := h.svc.GetConnection(ctx, r.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&channelv1.UpdateConnectionResponse{
		Connection: connectionToProto(conn),
	}), nil
}

func (h *ConnectionHandler) DeleteConnection(
	ctx context.Context,
	req *connect.Request[channelv1.DeleteConnectionRequest],
) (*connect.Response[channelv1.DeleteConnectionResponse], error) {
	if req.Msg.GetId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is required"))
	}
	if err := h.svc.DeleteConnection(ctx, req.Msg.GetId()); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&channelv1.DeleteConnectionResponse{}), nil
}
