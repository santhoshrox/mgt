package api

import "time"

// API DTOs. Kept separate from db.* so we can evolve persistence without
// breaking the wire format.

type MeResponse struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	AvatarURL string `json:"avatar_url"`
}

type RepoDTO struct {
	ID            int64  `json:"id"`
	Owner         string `json:"owner"`
	Name          string `json:"name"`
	FullName      string `json:"full_name"`
	DefaultBranch string `json:"default_branch"`
}

type StackDTO struct {
	ID          int64             `json:"id"`
	UserID      int64             `json:"user_id"`
	RepoID      int64             `json:"repo_id"`
	TrunkBranch string            `json:"trunk_branch"`
	Name        string            `json:"name"`
	Branches    []StackBranchDTO  `json:"branches"`
	CreatedAt   time.Time         `json:"created_at"`
}

type StackBranchDTO struct {
	ID         int64   `json:"id"`
	Name       string  `json:"name"`
	Parent     string  `json:"parent"` // resolved name for the CLI
	ParentID   *int64  `json:"parent_id,omitempty"`
	Position   int     `json:"position"`
	HeadSHA    string  `json:"head_sha,omitempty"`
	PRNumber   *int    `json:"pr_number,omitempty"`
}

type CreateStackInput struct {
	RepoID int64  `json:"repo_id"`
	Branch string `json:"branch"`        // initial top-of-stack branch name
	Trunk  string `json:"trunk,omitempty"` // override default branch
}

type AddBranchInput struct {
	Name   string `json:"name"`
	Parent string `json:"parent,omitempty"` // empty means trunk
}

type MoveBranchInput struct {
	Parent string `json:"parent,omitempty"`           // new parent name; empty means trunk
	Order  []string `json:"order,omitempty"`          // optional explicit order for siblings
}

// RebaseStep + RebasePlan describe the operations the CLI should perform to
// realise a server-side stack mutation. The CLI iterates the steps and runs
// `git rebase --onto` per branch.
type RebaseStep struct {
	Branch string `json:"branch"`
	Onto   string `json:"onto"`
}

type RebasePlan struct {
	Steps []RebaseStep `json:"steps"`
}

type CreatePRInput struct {
	Branch string `json:"branch"`
	Base   string `json:"base"`
	Title  string `json:"title"`
	Body   string `json:"body,omitempty"`
	Draft  bool   `json:"draft,omitempty"`
}

type PullRequestDTO struct {
	ID              int64     `json:"id"`
	Number          int       `json:"number"`
	Title           string    `json:"title"`
	Body            string    `json:"body"`
	State           string    `json:"state"`
	Merged          bool      `json:"merged"`
	Draft           bool      `json:"draft"`
	AuthorLogin     string    `json:"author_login"`
	AuthorAvatarURL string    `json:"author_avatar_url"`
	HeadBranch      string    `json:"head_branch"`
	BaseBranch      string    `json:"base_branch"`
	CIState         string    `json:"ci_state"`
	MergeableState  string    `json:"mergeable_state"`
	Additions       int       `json:"additions"`
	Deletions       int       `json:"deletions"`
	Comments        int       `json:"comments"`
	HTMLURL         string    `json:"html_url"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type DescribeResponse struct {
	Body string `json:"body"`
}

type EnqueueMergeInput struct {
	PRNumber int    `json:"pr_number"`
	Method   string `json:"method,omitempty"` // squash, merge, rebase
}

type MergeQueueEntryDTO struct {
	ID         int64     `json:"id"`
	PRNumber   int       `json:"pr_number"`
	State      string    `json:"state"`
	Position   int       `json:"position"`
	Attempts   int       `json:"attempts"`
	LastError  string    `json:"last_error,omitempty"`
	EnqueuedAt time.Time `json:"enqueued_at"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}
