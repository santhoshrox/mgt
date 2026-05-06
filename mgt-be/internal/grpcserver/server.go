// Package grpcserver exposes the same domain operations as the REST server,
// but over gRPC. It is the protocol used by mgt-cli; the React UI keeps
// talking to the REST endpoints under /api.
package grpcserver

import (
	"context"
	"errors"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	mgtv1 "github.com/santhosh/mgt-proto/gen/mgt/v1"

	"github.com/santhosh/mgt-be/internal/ai"
	"github.com/santhosh/mgt-be/internal/auth"
	"github.com/santhosh/mgt-be/internal/config"
	"github.com/santhosh/mgt-be/internal/core"
	"github.com/santhosh/mgt-be/internal/crypto"
	"github.com/santhosh/mgt-be/internal/db"
	"github.com/santhosh/mgt-be/internal/github"
	"github.com/santhosh/mgt-be/internal/search"
)

// Server implements MgtServiceServer. It is a thin facade over the same
// dependencies the REST server uses.
type Server struct {
	mgtv1.UnimplementedMgtServiceServer

	cfg    *config.Config
	db     *db.DB
	sealer *crypto.Sealer
	device *auth.DeviceFlow
	ai     *ai.Client
	search *search.Client
	queue  core.QueueController

	gs *grpc.Server
}

// New constructs a *grpc.Server with the auth interceptor wired up and the
// MgtService registered on it. Call Serve(ln) to start.
func New(c *core.Core) (*Server, error) {
	if c == nil {
		return nil, errors.New("nil core")
	}
	s := &Server{
		cfg:    c.Cfg,
		db:     c.DB,
		sealer: c.Sealer,
		device: c.Device,
		ai:     c.AI,
		search: c.Search,
		queue:  c.Queue,
	}
	s.gs = grpc.NewServer(
		grpc.ChainUnaryInterceptor(s.unaryAuthInterceptor),
	)
	mgtv1.RegisterMgtServiceServer(s.gs, s)
	return s, nil
}

func (s *Server) Serve(ln net.Listener) error  { return s.gs.Serve(ln) }
func (s *Server) GracefulStop()                { s.gs.GracefulStop() }
func (s *Server) Stop()                        { s.gs.Stop() }

// ── ghClient builds an authenticated GitHub client for a user. ───────────
func (s *Server) ghClient(u db.User) (*github.Client, error) {
	tok, err := s.sealer.Open(u.EncryptedOAuthToken)
	if err != nil {
		return nil, err
	}
	return github.New(tok), nil
}

// repoCheck loads a repo by id and verifies it exists.
func (s *Server) repoCheck(ctx context.Context, id int64) (db.Repository, error) {
	if id <= 0 {
		return db.Repository{}, status.Error(codes.InvalidArgument, "repo_id required")
	}
	r, err := s.db.GetRepository(ctx, id)
	if err != nil {
		return db.Repository{}, status.Error(codes.NotFound, "repo not found")
	}
	return r, nil
}

// metadataAuthority pulls Bearer tokens out of the metadata. Unauthenticated
// methods (DeviceStart / DevicePoll) skip this.
func bearerFromMD(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	for _, key := range []string{"authorization", "Authorization"} {
		if v := md.Get(key); len(v) > 0 {
			tok := v[0]
			if len(tok) > 7 && (tok[:7] == "Bearer " || tok[:7] == "bearer ") {
				return tok[7:]
			}
			return tok
		}
	}
	return ""
}
