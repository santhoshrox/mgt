package api

import (
	"net/http"

	"github.com/santhoshrox/mgt-be/internal/db"
)

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	writeJSON(w, http.StatusOK, MeResponse{ID: u.ID, Login: u.Login, AvatarURL: u.AvatarURL})
}

// listRepos returns the repositories registered with mgt-be that the user
// can see (currently: every repo they've synced).
func (s *Server) listRepos(w http.ResponseWriter, r *http.Request) {
	repos, err := s.db.ListRepositories(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]RepoDTO, 0, len(repos))
	for _, repo := range repos {
		out = append(out, RepoDTO{
			ID: repo.ID, Owner: repo.Owner, Name: repo.Name,
			FullName: repo.FullName(), DefaultBranch: repo.DefaultBranch,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// syncRepos fetches the user's repos from GitHub and upserts them into the
// local catalog. Webhook registration happens lazily on the first PR/queue
// action against the repo (or via a future explicit "install" endpoint).
func (s *Server) syncRepos(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	gh, err := s.ghClientForUser(u)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	ghRepos, err := gh.ListRepos()
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	out := make([]RepoDTO, 0, len(ghRepos))
	for _, r0 := range ghRepos {
		userID := u.ID
		repo, err := s.db.UpsertRepository(r.Context(), db.Repository{
			GitHubID:          r0.ID,
			Owner:             r0.Owner.Login,
			Name:              r0.Name,
			DefaultBranch:     defaultStr(r0.DefaultBranch, "main"),
			InstalledByUserID: &userID,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		out = append(out, RepoDTO{
			ID: repo.ID, Owner: repo.Owner, Name: repo.Name,
			FullName: repo.FullName(), DefaultBranch: repo.DefaultBranch,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func defaultStr(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
