package grpcserver

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/santhoshrox/mgt-be/internal/auth"
	"github.com/santhoshrox/mgt-be/internal/db"
)

// userCtxKey scopes the authenticated user attached by the interceptor.
type userCtxKey struct{}

func userFromCtx(ctx context.Context) (db.User, bool) {
	u, ok := ctx.Value(userCtxKey{}).(db.User)
	return u, ok
}

// publicMethods are dispatched without an authenticated user. They are the
// device-flow handshake methods plus standard gRPC reflection / health.
var publicMethods = map[string]struct{}{
	"/mgt.v1.MgtService/DeviceStart": {},
	"/mgt.v1.MgtService/DevicePoll":  {},
}

func (s *Server) unaryAuthInterceptor(
	ctx context.Context,
	req any,
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (any, error) {
	if _, ok := publicMethods[info.FullMethod]; ok {
		return handler(ctx, req)
	}
	tok := bearerFromMD(ctx)
	if tok == "" {
		return nil, status.Error(codes.Unauthenticated, "authorization required")
	}
	id, err := s.db.UserIDByTokenHash(ctx, auth.HashAPIToken(tok))
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid token")
	}
	u, err := s.db.GetUserByID(ctx, id)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "user not found")
	}
	return handler(context.WithValue(ctx, userCtxKey{}, u), req)
}
