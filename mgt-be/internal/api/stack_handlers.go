package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/santhoshrox/mgt-be/internal/db"
	"github.com/santhoshrox/mgt-be/internal/stacks"
)

func (s *Server) listStacks(w http.ResponseWriter, r *http.Request) {
	repo, err := s.repoFromCtx(r)
	if err != nil {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	u := userFrom(r)
	rows, err := s.db.ListStacks(r.Context(), u.ID, repo.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]StackDTO, 0, len(rows))
	for _, st := range rows {
		branches, _ := s.db.ListStackBranches(r.Context(), st.ID)
		out = append(out, toStackDTO(st, branches))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) getStackByBranch(w http.ResponseWriter, r *http.Request) {
	repo, err := s.repoFromCtx(r)
	if err != nil {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	u := userFrom(r)
	branch := r.URL.Query().Get("name")
	if branch == "" {
		writeError(w, http.StatusBadRequest, "name required")
		return
	}
	st, branches, err := s.db.GetStackByBranch(r.Context(), u.ID, repo.ID, branch)
	if err != nil {
		if db.IsNotFound(err) {
			// Helpful "empty stack" response: at least let the CLI know the trunk.
			writeJSON(w, http.StatusOK, StackDTO{TrunkBranch: repo.DefaultBranch})
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toStackDTO(st, branches))
}

func (s *Server) createStack(w http.ResponseWriter, r *http.Request) {
	repo, err := s.repoFromCtx(r)
	if err != nil {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	u := userFrom(r)
	var in CreateStackInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	trunk := in.Trunk
	if trunk == "" {
		trunk = repo.DefaultBranch
	}

	newStack, err := s.db.CreateStack(r.Context(), db.Stack{
		UserID: u.ID, RepoID: repo.ID, TrunkBranch: trunk,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if in.Branch != "" && in.Branch != trunk {
		if _, err := s.db.AddStackBranch(r.Context(), db.StackBranch{
			StackID:  newStack.ID,
			Name:     in.Branch,
			Position: 0,
		}); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	branches, _ := s.db.ListStackBranches(r.Context(), newStack.ID)
	writeJSON(w, http.StatusCreated, toStackDTO(newStack, branches))
}

func (s *Server) addBranch(w http.ResponseWriter, r *http.Request) {
	stackID, err := parseInt64(chi.URLParam(r, "stackID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad stack id")
		return
	}
	var in AddBranchInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	branches, err := s.db.ListStackBranches(r.Context(), stackID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var parentID *int64
	for _, b := range branches {
		if b.Name == in.Parent {
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
	added, err := s.db.AddStackBranch(r.Context(), db.StackBranch{
		StackID: stackID, Name: in.Name, ParentID: parentID, Position: pos,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, StackBranchDTO{
		ID: added.ID, Name: added.Name, Parent: in.Parent, ParentID: parentID, Position: pos,
	})
}

// moveBranch implements extract / reorder / move. It returns a RebasePlan the
// CLI executes locally.
func (s *Server) moveBranch(w http.ResponseWriter, r *http.Request) {
	stackID, err := parseInt64(chi.URLParam(r, "stackID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad stack id")
		return
	}
	branchID, err := parseInt64(chi.URLParam(r, "branchID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad branch id")
		return
	}
	var in MoveBranchInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	branches, err := s.db.ListStackBranches(r.Context(), stackID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	plan, err := stacks.PlanMove(branches, branchID, in.Parent)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	for _, upd := range plan.DBUpdates {
		if err := s.db.UpdateStackBranchParent(r.Context(), upd.BranchID, upd.ParentID, upd.Position); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	steps := make([]RebaseStep, 0, len(plan.RebaseSteps))
	for _, s := range plan.RebaseSteps {
		steps = append(steps, RebaseStep{Branch: s.Branch, Onto: s.Onto})
	}
	writeJSON(w, http.StatusOK, RebasePlan{Steps: steps})
}

func (s *Server) deleteBranch(w http.ResponseWriter, r *http.Request) {
	branchID, err := parseInt64(chi.URLParam(r, "branchID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad branch id")
		return
	}
	if err := s.db.DeleteStackBranch(r.Context(), branchID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── Helpers ────────────────────────────────────────────────────────────────

func toStackDTO(st db.Stack, branches []db.StackBranch) StackDTO {
	byID := map[int64]db.StackBranch{}
	for _, b := range branches {
		byID[b.ID] = b
	}
	out := StackDTO{
		ID: st.ID, UserID: st.UserID, RepoID: st.RepoID,
		TrunkBranch: st.TrunkBranch, Name: st.Name, CreatedAt: st.CreatedAt,
	}
	for _, b := range branches {
		parent := st.TrunkBranch
		if b.ParentID != nil {
			if p, ok := byID[*b.ParentID]; ok {
				parent = p.Name
			}
		}
		out.Branches = append(out.Branches, StackBranchDTO{
			ID: b.ID, Name: b.Name, Parent: parent, ParentID: b.ParentID,
			Position: b.Position, HeadSHA: b.HeadSHA,
		})
	}
	return out
}

