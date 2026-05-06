package api

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

func (s *Server) listMergeQueue(w http.ResponseWriter, r *http.Request) {
	repo, err := s.repoFromCtx(r)
	if err != nil {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	rows, err := s.db.ListMergeQueue(r.Context(), repo.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]MergeQueueEntryDTO, 0, len(rows))
	for _, e := range rows {
		num := 0
		if prRow, prErr := getPRByID(r, s, e.PRID); prErr == nil {
			num = prRow.Number
		}
		out = append(out, MergeQueueEntryDTO{
			ID: e.ID, PRNumber: num, State: e.State, Position: e.Position,
			Attempts: e.Attempts, LastError: e.LastError, EnqueuedAt: e.EnqueuedAt,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) enqueueMerge(w http.ResponseWriter, r *http.Request) {
	repo, err := s.repoFromCtx(r)
	if err != nil {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	u := userFrom(r)
	var in EnqueueMergeInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	pr, err := s.db.GetPullRequest(r.Context(), repo.ID, in.PRNumber)
	if err != nil {
		// Sync from GitHub if we don't have it yet.
		gh, ghErr := s.ghClientForUser(u)
		if ghErr != nil {
			writeError(w, http.StatusInternalServerError, ghErr.Error())
			return
		}
		ghPR, ghPRErr := gh.GetPR(repo.Owner, repo.Name, in.PRNumber)
		if ghPRErr != nil {
			writeError(w, http.StatusBadGateway, ghPRErr.Error())
			return
		}
		pr, err = s.upsertPRFromGitHub(r, repo, ghPR)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	entry, err := s.db.EnqueueMerge(r.Context(), repo.ID, pr.ID, u.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if s.queue != nil {
		s.queue.Notify(repo.ID)
	}
	writeJSON(w, http.StatusCreated, MergeQueueEntryDTO{
		ID: entry.ID, PRNumber: pr.Number, State: entry.State, Position: entry.Position,
		Attempts: entry.Attempts, LastError: entry.LastError, EnqueuedAt: entry.EnqueuedAt,
	})
}

func (s *Server) cancelMerge(w http.ResponseWriter, r *http.Request) {
	repo, err := s.repoFromCtx(r)
	if err != nil {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "entryID"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad entry id")
		return
	}
	if err := s.db.CancelMergeQueueEntry(r.Context(), repo.ID, id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// getPRByID is a tiny convenience that wraps the raw DB call; declared inline
// because PR rows are looked up here in only one place.
func getPRByID(r *http.Request, s *Server, prID int64) (struct{ Number int }, error) {
	row := struct{ Number int }{}
	err := s.db.Pool.QueryRow(r.Context(), `SELECT number FROM pull_requests WHERE id=$1`, prID).Scan(&row.Number)
	return row, err
}
