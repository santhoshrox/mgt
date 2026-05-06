package api

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/santhoshrox/mgt-be/internal/ai"
	"github.com/santhoshrox/mgt-be/internal/db"
	"github.com/santhoshrox/mgt-be/internal/github"
)

func (s *Server) listPRs(w http.ResponseWriter, r *http.Request) {
	repo, err := s.repoFromCtx(r)
	if err != nil {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	state := r.URL.Query().Get("state")
	author := r.URL.Query().Get("author")
	q := r.URL.Query().Get("q")

	rows, err := s.db.ListPullRequests(r.Context(), repo.ID, state, author)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if q != "" && s.search.Enabled() {
		nums, err := s.search.Query(r.Context(), repo.ID, q, 100)
		if err == nil {
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
	out := make([]PullRequestDTO, 0, len(rows))
	for _, pr := range rows {
		out = append(out, prDTO(pr))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) getPR(w http.ResponseWriter, r *http.Request) {
	repo, err := s.repoFromCtx(r)
	if err != nil {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	num, err := strconv.Atoi(chi.URLParam(r, "number"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad number")
		return
	}
	pr, err := s.db.GetPullRequest(r.Context(), repo.ID, num)
	if err != nil {
		writeError(w, http.StatusNotFound, "pr not found")
		return
	}
	writeJSON(w, http.StatusOK, prDTO(pr))
}

func (s *Server) createPR(w http.ResponseWriter, r *http.Request) {
	repo, err := s.repoFromCtx(r)
	if err != nil {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	u := userFrom(r)
	gh, err := s.ghClientForUser(u)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var in CreatePRInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if in.Branch == "" || in.Base == "" || in.Title == "" {
		writeError(w, http.StatusBadRequest, "branch, base, title required")
		return
	}

	body := in.Body
	useAI := r.URL.Query().Get("ai") == "1"
	if useAI && s.ai.Configured() && body == "" {
		body, _ = s.aiBody(repo, gh, in.Branch, in.Base)
	}

	// Upsert PR: try to find an existing one for this head branch.
	existing, found, err := gh.FindPRByHeadBranch(repo.Owner, repo.Name, in.Branch)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	var pr github.PR
	if found {
		patch := map[string]any{"title": in.Title}
		if body != "" {
			patch["body"] = body
		}
		pr, err = gh.UpdatePR(repo.Owner, repo.Name, existing.Number, patch)
	} else {
		pr, err = gh.CreatePR(repo.Owner, repo.Name, github.CreatePRInput{
			Title: in.Title, Head: in.Branch, Base: in.Base, Body: body, Draft: in.Draft,
		})
	}
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	stored, err := s.upsertPRFromGitHub(r, repo, pr)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, prDTO(stored))
}

func (s *Server) describePR(w http.ResponseWriter, r *http.Request) {
	if !s.ai.Configured() {
		writeError(w, http.StatusPreconditionFailed, "AI not configured on server (set LLM_API_KEY)")
		return
	}
	repo, err := s.repoFromCtx(r)
	if err != nil {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	num, err := strconv.Atoi(chi.URLParam(r, "number"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad number")
		return
	}
	gh, err := s.ghClientForUser(userFrom(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	pr, err := gh.GetPR(repo.Owner, repo.Name, num)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	body, err := s.aiBody(repo, gh, pr.Head.Ref, pr.Base.Ref)
	if err != nil {
		if errors.Is(err, ai.ErrNotConfigured) {
			writeError(w, http.StatusPreconditionFailed, err.Error())
			return
		}
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	if body != "" {
		_, _ = gh.UpdatePR(repo.Owner, repo.Name, num, map[string]any{"body": body})
	}
	writeJSON(w, http.StatusOK, DescribeResponse{Body: body})
}

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

// upsertPRFromGitHub mirrors the GitHub PR shape into Postgres + OpenSearch.
func (s *Server) upsertPRFromGitHub(r *http.Request, repo db.Repository, pr github.PR) (db.PullRequest, error) {
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
	stored, err := s.db.UpsertPullRequest(r.Context(), row)
	if err != nil {
		return db.PullRequest{}, err
	}
	go s.search.IndexPR(r.Context(), stored)
	return stored, nil
}

func prDTO(pr db.PullRequest) PullRequestDTO {
	updated := time.Time{}
	if pr.UpdatedAtGH != nil {
		updated = *pr.UpdatedAtGH
	}
	return PullRequestDTO{
		ID: pr.ID, Number: pr.Number, Title: pr.Title, Body: pr.Body,
		State: pr.State, Merged: pr.Merged, Draft: pr.Draft,
		AuthorLogin: pr.AuthorLogin, AuthorAvatarURL: pr.AuthorAvatarURL,
		HeadBranch: pr.HeadBranch, BaseBranch: pr.BaseBranch,
		CIState: pr.CIState, MergeableState: pr.MergeableState,
		Additions: pr.Additions, Deletions: pr.Deletions, Comments: pr.Comments,
		HTMLURL: pr.HTMLURL, UpdatedAt: updated,
	}
}
