package grpcserver

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	mgtv1 "github.com/santhoshrox/mgt-proto/gen/mgt/v1"

	"github.com/santhoshrox/mgt-be/internal/ai"
	"github.com/santhoshrox/mgt-be/internal/db"
	"github.com/santhoshrox/mgt-be/internal/github"
)

func (s *Server) ListPullRequests(ctx context.Context, in *mgtv1.ListPullRequestsRequest) (*mgtv1.ListPullRequestsResponse, error) {
	repo, err := s.repoCheck(ctx, in.GetRepoId())
	if err != nil {
		return nil, err
	}
	rows, err := s.db.ListPullRequests(ctx, repo.ID, in.GetState(), in.GetAuthor())
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if q := in.GetQ(); q != "" && s.search.Enabled() {
		nums, qerr := s.search.Query(ctx, repo.ID, q, 100)
		if qerr == nil {
			set := map[int]struct{}{}
			for _, n := range nums {
				set[n] = struct{}{}
			}
			filtered := rows[:0]
			for _, pr := range rows {
				if _, ok := set[pr.Number]; ok {
					filtered = append(filtered, pr)
				}
			}
			rows = filtered
		}
	}
	out := &mgtv1.ListPullRequestsResponse{Pulls: make([]*mgtv1.PullRequest, 0, len(rows))}
	for _, pr := range rows {
		out.Pulls = append(out.Pulls, prPB(pr))
	}
	return out, nil
}

func (s *Server) CreatePullRequest(ctx context.Context, in *mgtv1.CreatePullRequestRequest) (*mgtv1.PullRequest, error) {
	u, _ := userFromCtx(ctx)
	repo, err := s.repoCheck(ctx, in.GetRepoId())
	if err != nil {
		return nil, err
	}
	if in.GetBranch() == "" || in.GetBase() == "" || in.GetTitle() == "" {
		return nil, status.Error(codes.InvalidArgument, "branch, base, title required")
	}
	gh, err := s.ghClient(u)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	body := in.GetBody()
	if in.GetAi() && s.ai.Configured() && body == "" {
		body, _ = s.aiBody(repo, gh, in.GetBranch(), in.GetBase())
	}

	existing, found, err := gh.FindPRByHeadBranch(repo.Owner, repo.Name, in.GetBranch())
	if err != nil {
		return nil, status.Error(codes.Unavailable, err.Error())
	}
	var pr github.PR
	if found {
		patch := map[string]any{"title": in.GetTitle()}
		if body != "" {
			patch["body"] = body
		}
		pr, err = gh.UpdatePR(repo.Owner, repo.Name, existing.Number, patch)
	} else {
		pr, err = gh.CreatePR(repo.Owner, repo.Name, github.CreatePRInput{
			Title: in.GetTitle(), Head: in.GetBranch(), Base: in.GetBase(),
			Body: body, Draft: in.GetDraft(),
		})
	}
	if err != nil {
		return nil, status.Error(codes.Unavailable, err.Error())
	}
	stored, err := s.upsertPR(ctx, repo, pr)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return prPB(stored), nil
}

func (s *Server) DescribePullRequest(ctx context.Context, in *mgtv1.DescribePullRequestRequest) (*mgtv1.DescribeResponse, error) {
	if !s.ai.Configured() {
		return nil, status.Error(codes.FailedPrecondition, "AI not configured on server (set LLM_API_KEY)")
	}
	u, _ := userFromCtx(ctx)
	repo, err := s.repoCheck(ctx, in.GetRepoId())
	if err != nil {
		return nil, err
	}
	num := int(in.GetNumber())
	if num <= 0 {
		return nil, status.Error(codes.InvalidArgument, "number required")
	}
	gh, err := s.ghClient(u)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	pr, err := gh.GetPR(repo.Owner, repo.Name, num)
	if err != nil {
		return nil, status.Error(codes.Unavailable, err.Error())
	}
	body, err := s.aiBody(repo, gh, pr.Head.Ref, pr.Base.Ref)
	if err != nil {
		if errors.Is(err, ai.ErrNotConfigured) {
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		}
		return nil, status.Error(codes.Unavailable, err.Error())
	}
	if body != "" {
		_, _ = gh.UpdatePR(repo.Owner, repo.Name, num, map[string]any{"body": body})
	}
	return &mgtv1.DescribeResponse{Body: body}, nil
}

// ── shared helpers (mirror api package) ──────────────────────────────────

func (s *Server) aiBody(repo db.Repository, gh *github.Client, branch, base string) (string, error) {
	template := gh.FindPRTemplate(repo.Owner, repo.Name)
	if template == "" {
		template = ai.DefaultPRTemplate
	}
	diff, err := gh.CompareDiff(repo.Owner, repo.Name, base, branch)
	if err != nil {
		return "", err
	}
	return s.ai.FillPRBody(template, diff, "")
}

func (s *Server) upsertPR(ctx context.Context, repo db.Repository, pr github.PR) (db.PullRequest, error) {
	updatedAt := pr.UpdatedAt
	row := db.PullRequest{
		RepoID:          repo.ID,
		Number:          pr.Number,
		Title:           pr.Title,
		Body:            pr.Body,
		State:           pr.State,
		Merged:          pr.Merged,
		Draft:           pr.Draft,
		AuthorLogin:     pr.User.Login,
		AuthorAvatarURL: pr.User.AvatarURL,
		HeadBranch:      pr.Head.Ref,
		BaseBranch:      pr.Base.Ref,
		HeadSHA:         pr.Head.SHA,
		MergeableState:  pr.MergeableState,
		Additions:       pr.Additions,
		Deletions:       pr.Deletions,
		Comments:        pr.Comments,
		HTMLURL:         pr.HTMLURL,
		UpdatedAtGH:     &updatedAt,
	}
	stored, err := s.db.UpsertPullRequest(ctx, row)
	if err != nil {
		return db.PullRequest{}, err
	}
	go s.search.IndexPR(context.Background(), stored)
	return stored, nil
}
