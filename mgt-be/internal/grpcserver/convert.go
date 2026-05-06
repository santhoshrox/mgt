package grpcserver

import (
	"google.golang.org/protobuf/types/known/timestamppb"

	mgtv1 "github.com/santhoshrox/mgt-proto/gen/mgt/v1"

	"github.com/santhoshrox/mgt-be/internal/db"
)

func repoPB(r db.Repository) *mgtv1.Repo {
	return &mgtv1.Repo{
		Id:            r.ID,
		Owner:         r.Owner,
		Name:          r.Name,
		FullName:      r.FullName(),
		DefaultBranch: r.DefaultBranch,
	}
}

func stackPB(st db.Stack, branches []db.StackBranch) *mgtv1.Stack {
	byID := map[int64]db.StackBranch{}
	for _, b := range branches {
		byID[b.ID] = b
	}
	out := &mgtv1.Stack{
		Id:          st.ID,
		TrunkBranch: st.TrunkBranch,
		Name:        st.Name,
	}
	if !st.CreatedAt.IsZero() {
		out.CreatedAt = timestamppb.New(st.CreatedAt)
	}
	for _, b := range branches {
		parent := st.TrunkBranch
		if b.ParentID != nil {
			if p, ok := byID[*b.ParentID]; ok {
				parent = p.Name
			}
		}
		out.Branches = append(out.Branches, &mgtv1.StackBranch{
			Id:       b.ID,
			Name:     b.Name,
			Parent:   parent,
			Position: int32(b.Position),
			HeadSha:  b.HeadSHA,
		})
	}
	return out
}

func prPB(pr db.PullRequest) *mgtv1.PullRequest {
	out := &mgtv1.PullRequest{
		Id:              pr.ID,
		Number:          int32(pr.Number),
		Title:           pr.Title,
		Body:            pr.Body,
		State:           pr.State,
		Merged:          pr.Merged,
		Draft:           pr.Draft,
		AuthorLogin:     pr.AuthorLogin,
		AuthorAvatarUrl: pr.AuthorAvatarURL,
		HeadBranch:      pr.HeadBranch,
		BaseBranch:      pr.BaseBranch,
		CiState:         pr.CIState,
		MergeableState:  pr.MergeableState,
		Additions:       int32(pr.Additions),
		Deletions:       int32(pr.Deletions),
		Comments:        int32(pr.Comments),
		HtmlUrl:         pr.HTMLURL,
	}
	if pr.UpdatedAtGH != nil {
		out.UpdatedAt = timestamppb.New(*pr.UpdatedAtGH)
	}
	return out
}

func mqEntryPB(e db.MergeQueueEntry, prNumber int) *mgtv1.MergeQueueEntry {
	out := &mgtv1.MergeQueueEntry{
		Id:        e.ID,
		PrNumber:  int32(prNumber),
		State:     e.State,
		Position:  int32(e.Position),
		Attempts:  int32(e.Attempts),
		LastError: e.LastError,
	}
	if !e.EnqueuedAt.IsZero() {
		out.EnqueuedAt = timestamppb.New(e.EnqueuedAt)
	}
	return out
}
