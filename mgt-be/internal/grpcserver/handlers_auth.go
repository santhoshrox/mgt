package grpcserver

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	mgtv1 "github.com/santhosh/mgt-proto/gen/mgt/v1"
)

// DeviceStart begins the CLI device-flow handshake. Mirrors REST /auth/cli/start.
func (s *Server) DeviceStart(ctx context.Context, _ *emptypb.Empty) (*mgtv1.DeviceStartResponse, error) {
	uc, st, err := s.device.Start()
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &mgtv1.DeviceStartResponse{
		UserCode:        uc,
		State:           st,
		VerificationUrl: s.cfg.BaseURL + "/auth/github/login?cli_state=" + st,
	}, nil
}

// DevicePoll blocks (server-side, up to 60s) until the user completes the
// browser flow, then returns the freshly minted API token.
func (s *Server) DevicePoll(ctx context.Context, in *mgtv1.DevicePollRequest) (*mgtv1.DevicePollResponse, error) {
	if in.GetState() == "" {
		return nil, status.Error(codes.InvalidArgument, "state required")
	}
	pollCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	tok, _, err := s.device.Poll(pollCtx, in.GetState())
	if err != nil {
		return nil, status.Error(codes.DeadlineExceeded, err.Error())
	}
	return &mgtv1.DevicePollResponse{Token: tok}, nil
}
