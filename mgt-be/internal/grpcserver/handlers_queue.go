package grpcserver

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	mgtv1 "github.com/santhoshrox/mgt-proto/gen/mgt/v1"
)

func (s *Server) ListMergeQueue(ctx context.Context, in *mgtv1.ListMergeQueueRequest) (*mgtv1.ListMergeQueueResponse, error) {
	repo, err := s.repoCheck(ctx, in.GetRepoId())
	if err != nil {
		return nil, err
	}
	rows, err := s.db.ListMergeQueue(ctx, repo.ID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	out := &mgtv1.ListMergeQueueResponse{Entries: make([]*mgtv1.MergeQueueEntry, 0, len(rows))}
	for _, e := range rows {
		num := 0
		if pr, prErr := s.db.GetPullRequestByID(ctx, e.PRID); prErr == nil {
			num = pr.Number
		}
		out.Entries = append(out.Entries, mqEntryPB(e, num))
	}
	return out, nil
}

func (s *Server) EnqueueMerge(ctx context.Context, in *mgtv1.EnqueueMergeRequest) (*mgtv1.MergeQueueEntry, error) {
	u, _ := userFromCtx(ctx)
	repo, err := s.repoCheck(ctx, in.GetRepoId())
	if err != nil {
		return nil, err
	}
	num := int(in.GetPrNumber())
	if num <= 0 {
		return nil, status.Error(codes.InvalidArgument, "pr_number required")
	}
	pr, err := s.db.GetPullRequest(ctx, repo.ID, num)
	if err != nil {
		gh, ghErr := s.ghClient(u)
		if ghErr != nil {
			return nil, status.Error(codes.Internal, ghErr.Error())
		}
		ghPR, ghPRErr := gh.GetPR(repo.Owner, repo.Name, num)
		if ghPRErr != nil {
			return nil, status.Error(codes.Unavailable, ghPRErr.Error())
		}
		pr, err = s.upsertPR(ctx, repo, ghPR)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	entry, err := s.db.EnqueueMerge(ctx, repo.ID, pr.ID, u.ID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if s.queue != nil {
		s.queue.Notify(repo.ID)
	}
	return mqEntryPB(entry, pr.Number), nil
}

func (s *Server) CancelMerge(ctx context.Context, in *mgtv1.CancelMergeRequest) (*emptypb.Empty, error) {
	repo, err := s.repoCheck(ctx, in.GetRepoId())
	if err != nil {
		return nil, err
	}
	if err := s.db.CancelMergeQueueEntry(ctx, repo.ID, in.GetEntryId()); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &emptypb.Empty{}, nil
}
