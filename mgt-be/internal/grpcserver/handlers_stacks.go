package grpcserver

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	mgtv1 "github.com/santhoshrox/mgt-proto/gen/mgt/v1"

	"github.com/santhoshrox/mgt-be/internal/db"
	"github.com/santhoshrox/mgt-be/internal/stacks"
)

func (s *Server) ListStacks(ctx context.Context, in *mgtv1.ListStacksRequest) (*mgtv1.ListStacksResponse, error) {
	u, _ := userFromCtx(ctx)
	if _, err := s.repoCheck(ctx, in.GetRepoId()); err != nil {
		return nil, err
	}
	rows, err := s.db.ListStacks(ctx, u.ID, in.GetRepoId())
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	out := &mgtv1.ListStacksResponse{Stacks: make([]*mgtv1.Stack, 0, len(rows))}
	for _, st := range rows {
		branches, _ := s.db.ListStackBranches(ctx, st.ID)
		out.Stacks = append(out.Stacks, stackPB(st, branches))
	}
	return out, nil
}

func (s *Server) StackByBranch(ctx context.Context, in *mgtv1.StackByBranchRequest) (*mgtv1.Stack, error) {
	u, _ := userFromCtx(ctx)
	repo, err := s.repoCheck(ctx, in.GetRepoId())
	if err != nil {
		return nil, err
	}
	if in.GetBranch() == "" {
		return nil, status.Error(codes.InvalidArgument, "branch required")
	}
	st, branches, err := s.db.GetStackByBranch(ctx, u.ID, repo.ID, in.GetBranch())
	if err != nil {
		if db.IsNotFound(err) {
			// Mirror REST: return an "empty stack" with just the trunk so the
			// CLI can still answer "where am I" on a vanilla branch.
			return &mgtv1.Stack{TrunkBranch: repo.DefaultBranch}, nil
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return stackPB(st, branches), nil
}

func (s *Server) CreateStack(ctx context.Context, in *mgtv1.CreateStackRequest) (*mgtv1.Stack, error) {
	u, _ := userFromCtx(ctx)
	repo, err := s.repoCheck(ctx, in.GetRepoId())
	if err != nil {
		return nil, err
	}
	trunk := in.GetTrunk()
	if trunk == "" {
		trunk = repo.DefaultBranch
	}
	newStack, err := s.db.CreateStack(ctx, db.Stack{
		UserID: u.ID, RepoID: repo.ID, TrunkBranch: trunk,
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if in.GetBranch() != "" && in.GetBranch() != trunk {
		if _, err := s.db.AddStackBranch(ctx, db.StackBranch{
			StackID: newStack.ID, Name: in.GetBranch(), Position: 0,
		}); err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	branches, _ := s.db.ListStackBranches(ctx, newStack.ID)
	return stackPB(newStack, branches), nil
}

func (s *Server) AddBranch(ctx context.Context, in *mgtv1.AddBranchRequest) (*mgtv1.StackBranch, error) {
	if _, err := s.repoCheck(ctx, in.GetRepoId()); err != nil {
		return nil, err
	}
	stackID := in.GetStackId()
	if stackID <= 0 {
		return nil, status.Error(codes.InvalidArgument, "stack_id required")
	}
	branches, err := s.db.ListStackBranches(ctx, stackID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	var parentID *int64
	for _, b := range branches {
		if b.Name == in.GetParent() {
			id := b.ID
			parentID = &id
			break
		}
	}
	pos := 0
	for _, b := range branches {
		if b.Position >= pos {
			pos = b.Position + 1
		}
	}
	added, err := s.db.AddStackBranch(ctx, db.StackBranch{
		StackID: stackID, Name: in.GetName(), ParentID: parentID, Position: pos,
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &mgtv1.StackBranch{
		Id: added.ID, Name: added.Name, Parent: in.GetParent(), Position: int32(pos),
	}, nil
}

func (s *Server) MoveBranch(ctx context.Context, in *mgtv1.MoveBranchRequest) (*mgtv1.RebasePlan, error) {
	if _, err := s.repoCheck(ctx, in.GetRepoId()); err != nil {
		return nil, err
	}
	branches, err := s.db.ListStackBranches(ctx, in.GetStackId())
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	plan, err := stacks.PlanMove(branches, in.GetBranchId(), in.GetParent())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	for _, upd := range plan.DBUpdates {
		if err := s.db.UpdateStackBranchParent(ctx, upd.BranchID, upd.ParentID, upd.Position); err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	out := &mgtv1.RebasePlan{Steps: make([]*mgtv1.RebaseStep, 0, len(plan.RebaseSteps))}
	for _, st := range plan.RebaseSteps {
		out.Steps = append(out.Steps, &mgtv1.RebaseStep{Branch: st.Branch, Onto: st.Onto})
	}
	return out, nil
}

func (s *Server) DeleteBranch(ctx context.Context, in *mgtv1.DeleteBranchRequest) (*emptypb.Empty, error) {
	if _, err := s.repoCheck(ctx, in.GetRepoId()); err != nil {
		return nil, err
	}
	if err := s.db.DeleteStackBranch(ctx, in.GetBranchId()); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &emptypb.Empty{}, nil
}
