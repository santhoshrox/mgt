package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
)

// githubWebhook accepts GitHub events and bumps PR + merge-queue state. We
// dedupe by X-GitHub-Delivery so retries are safe.
func (s *Server) githubWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "read body")
		return
	}
	if !s.verifyWebhookSig(r, body) {
		writeError(w, http.StatusUnauthorized, "bad signature")
		return
	}
	deliveryID := r.Header.Get("X-GitHub-Delivery")
	event := r.Header.Get("X-GitHub-Event")
	if deliveryID == "" || event == "" {
		writeError(w, http.StatusBadRequest, "missing delivery/event headers")
		return
	}

	// Pull repo info up front (most events carry it).
	var envelope struct {
		Repository struct {
			ID       int64  `json:"id"`
			Name     string `json:"name"`
			FullName string `json:"full_name"`
			Owner    struct{ Login string `json:"login"` } `json:"owner"`
		} `json:"repository"`
		PullRequest struct {
			Number int `json:"number"`
		} `json:"pull_request"`
	}
	_ = json.Unmarshal(body, &envelope)

	var repoID *int64
	if envelope.Repository.ID != 0 {
		if repo, err := s.db.GetRepositoryByOwnerName(r.Context(), envelope.Repository.Owner.Login, envelope.Repository.Name); err == nil {
			id := repo.ID
			repoID = &id
		}
	}
	first, err := s.db.RecordWebhookEvent(r.Context(), deliveryID, event, repoID, body)
	if err != nil {
		slog.Warn("webhook: record", "err", err)
	}
	if !first {
		w.WriteHeader(http.StatusOK)
		return
	}

	switch event {
	case "pull_request":
		s.onPullRequest(r, envelope.Repository.Owner.Login, envelope.Repository.Name, envelope.PullRequest.Number)
	case "check_run", "check_suite", "status":
		if repoID != nil {
			if s.queue != nil {
				s.queue.Notify(*repoID)
			}
		}
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) verifyWebhookSig(r *http.Request, body []byte) bool {
	secret := s.cfg.GitHubWebhookSecret
	if secret == "" {
		return true // self-host with secret unset: skip (development only)
	}
	header := r.Header.Get("X-Hub-Signature-256")
	if len(header) < 8 || header[:7] != "sha256=" {
		return false
	}
	want, err := hex.DecodeString(header[7:])
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hmac.Equal(mac.Sum(nil), want)
}

// onPullRequest re-syncs a single PR using whichever user originally
// installed the repo (best-effort; the user must still have a valid OAuth
// token).
func (s *Server) onPullRequest(r *http.Request, owner, name string, number int) {
	repo, err := s.db.GetRepositoryByOwnerName(r.Context(), owner, name)
	if err != nil || repo.InstalledByUserID == nil {
		return
	}
	user, err := s.db.GetUserByID(r.Context(), *repo.InstalledByUserID)
	if err != nil {
		return
	}
	gh, err := s.ghClientForUser(user)
	if err != nil {
		return
	}
	pr, err := gh.GetPR(repo.Owner, repo.Name, number)
	if err != nil {
		slog.Warn("webhook: get pr", "err", err)
		return
	}
	if _, err := s.upsertPRFromGitHub(r, repo, pr); err != nil {
		slog.Warn("webhook: upsert pr", "err", err)
	}
}
