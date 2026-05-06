package grpcserver

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	mgtv1 "github.com/santhoshrox/mgt-proto/gen/mgt/v1"

	"github.com/santhoshrox/mgt-be/internal/db"
)

func (s *Server) Me(ctx context.Context, _ *emptypb.Empty) (*mgtv1.MeResponse, error) {
	u, ok := userFromCtx(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "no user")
	}
	return &mgtv1.MeResponse{Id: u.ID, Login: u.Login, AvatarUrl: u.AvatarURL}, nil
}

func (s *Server) ListRepos(ctx context.Context, _ *emptypb.Empty) (*mgtv1.ListReposResponse, error) {
	rows, err := s.db.ListRepositories(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	out := &mgtv1.ListReposResponse{Repos: make([]*mgtv1.Repo, 0, len(rows))}
	for _, r := range rows {
		out.Repos = append(out.Repos, repoPB(r))
	}
	return out, nil
}

func (s *Server) SyncRepos(ctx context.Context, _ *emptypb.Empty) (*mgtv1.ListReposResponse, error) {
	u, ok := userFromCtx(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "no user")
	}
	gh, err := s.ghClient(u)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	ghRepos, err := gh.ListRepos()
	if err != nil {
		return nil, status.Error(codes.Unavailable, err.Error())
	}
	out := &mgtv1.ListReposResponse{Repos: make([]*mgtv1.Repo, 0, len(ghRepos))}
	for _, r0 := range ghRepos {
		userID := u.ID
		def := r0.DefaultBranch
		if def == "" {
			def = "main"
		}
		repo, err := s.db.UpsertRepository(ctx, db.Repository{
			GitHubID:          r0.ID,
			Owner:             r0.Owner.Login,
			Name:              r0.Name,
			DefaultBranch:     def,
			InstalledByUserID: &userID,
		})
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		out.Repos = append(out.Repos, repoPB(repo))
	}
	return out, nil
}
