package db

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

// ── Domain types ──────────────────────────────────────────────────────────

type User struct {
	ID                  int64
	GitHubID            int64
	Login               string
	AvatarURL           string
	EncryptedOAuthToken string
	Scopes              string
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type Repository struct {
	ID                int64
	GitHubID          int64
	Owner             string
	Name              string
	DefaultBranch     string
	WebhookSecretEnc  string
	InstalledByUserID *int64
}

func (r Repository) FullName() string { return r.Owner + "/" + r.Name }

type Stack struct {
	ID          int64
	UserID      int64
	RepoID      int64
	Name        string
	TrunkBranch string
	CreatedAt   time.Time
}

type StackBranch struct {
	ID       int64
	StackID  int64
	Name     string
	ParentID *int64
	Position int
	HeadSHA  string
	PRID     *int64
}

type PullRequest struct {
	ID              int64
	RepoID          int64
	Number          int
	Title           string
	Body            string
	State           string
	Merged          bool
	Draft           bool
	AuthorLogin     string
	AuthorAvatarURL string
	HeadBranch      string
	BaseBranch      string
	HeadSHA         string
	MergeableState  string
	CIState         string
	Additions       int
	Deletions       int
	Comments        int
	HTMLURL         string
	UpdatedAtGH     *time.Time
	SyncedAt        time.Time
}

type MergeQueueEntry struct {
	ID          int64
	RepoID      int64
	PRID        int64
	UserID      int64
	State       string
	Position    int
	Attempts    int
	LastError   string
	LastEventAt time.Time
	EnqueuedAt  time.Time
	MergedAt    *time.Time
}

// ── Users ─────────────────────────────────────────────────────────────────

func (d *DB) UpsertUser(ctx context.Context, u User) (User, error) {
	const q = `
INSERT INTO users (github_id, login, avatar_url, encrypted_oauth_token, scopes)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (github_id) DO UPDATE
SET login = EXCLUDED.login,
    avatar_url = EXCLUDED.avatar_url,
    encrypted_oauth_token = EXCLUDED.encrypted_oauth_token,
    scopes = EXCLUDED.scopes,
    updated_at = NOW()
RETURNING id, created_at, updated_at`
	if err := d.Pool.QueryRow(ctx, q, u.GitHubID, u.Login, u.AvatarURL, u.EncryptedOAuthToken, u.Scopes).
		Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt); err != nil {
		return User{}, err
	}
	return u, nil
}

func (d *DB) GetUserByID(ctx context.Context, id int64) (User, error) {
	const q = `SELECT id, github_id, login, COALESCE(avatar_url,''), encrypted_oauth_token, COALESCE(scopes,''), created_at, updated_at FROM users WHERE id=$1`
	var u User
	err := d.Pool.QueryRow(ctx, q, id).Scan(&u.ID, &u.GitHubID, &u.Login, &u.AvatarURL, &u.EncryptedOAuthToken, &u.Scopes, &u.CreatedAt, &u.UpdatedAt)
	return u, err
}

// ── API tokens ────────────────────────────────────────────────────────────

func (d *DB) InsertAPIToken(ctx context.Context, userID int64, tokenHash []byte, label string) error {
	_, err := d.Pool.Exec(ctx, `INSERT INTO api_tokens (user_id, token_hash, label) VALUES ($1,$2,$3)`, userID, tokenHash, label)
	return err
}

func (d *DB) UserIDByTokenHash(ctx context.Context, tokenHash []byte) (int64, error) {
	const q = `SELECT user_id FROM api_tokens WHERE token_hash=$1 AND revoked_at IS NULL`
	var id int64
	err := d.Pool.QueryRow(ctx, q, tokenHash).Scan(&id)
	if err == nil {
		_, _ = d.Pool.Exec(ctx, `UPDATE api_tokens SET last_used_at=NOW() WHERE token_hash=$1`, tokenHash)
	}
	return id, err
}

func (d *DB) RevokeAPITokensForUser(ctx context.Context, userID int64) error {
	_, err := d.Pool.Exec(ctx, `UPDATE api_tokens SET revoked_at=NOW() WHERE user_id=$1 AND revoked_at IS NULL`, userID)
	return err
}

// ── Repositories ──────────────────────────────────────────────────────────

func (d *DB) UpsertRepository(ctx context.Context, r Repository) (Repository, error) {
	const q = `
INSERT INTO repositories (github_id, owner, name, default_branch, webhook_secret_enc, installed_by_user_id)
VALUES ($1,$2,$3,$4,$5,$6)
ON CONFLICT (github_id) DO UPDATE
SET owner=EXCLUDED.owner, name=EXCLUDED.name, default_branch=EXCLUDED.default_branch, updated_at=NOW()
RETURNING id`
	if err := d.Pool.QueryRow(ctx, q, r.GitHubID, r.Owner, r.Name, r.DefaultBranch, r.WebhookSecretEnc, r.InstalledByUserID).Scan(&r.ID); err != nil {
		return Repository{}, err
	}
	return r, nil
}

func (d *DB) ListRepositories(ctx context.Context) ([]Repository, error) {
	rows, err := d.Pool.Query(ctx, `SELECT id, github_id, owner, name, default_branch, COALESCE(webhook_secret_enc,''), installed_by_user_id FROM repositories ORDER BY owner, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Repository
	for rows.Next() {
		var r Repository
		if err := rows.Scan(&r.ID, &r.GitHubID, &r.Owner, &r.Name, &r.DefaultBranch, &r.WebhookSecretEnc, &r.InstalledByUserID); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (d *DB) GetRepository(ctx context.Context, id int64) (Repository, error) {
	const q = `SELECT id, github_id, owner, name, default_branch, COALESCE(webhook_secret_enc,''), installed_by_user_id FROM repositories WHERE id=$1`
	var r Repository
	err := d.Pool.QueryRow(ctx, q, id).Scan(&r.ID, &r.GitHubID, &r.Owner, &r.Name, &r.DefaultBranch, &r.WebhookSecretEnc, &r.InstalledByUserID)
	return r, err
}

func (d *DB) GetRepositoryByOwnerName(ctx context.Context, owner, name string) (Repository, error) {
	const q = `SELECT id, github_id, owner, name, default_branch, COALESCE(webhook_secret_enc,''), installed_by_user_id FROM repositories WHERE owner=$1 AND name=$2`
	var r Repository
	err := d.Pool.QueryRow(ctx, q, owner, name).Scan(&r.ID, &r.GitHubID, &r.Owner, &r.Name, &r.DefaultBranch, &r.WebhookSecretEnc, &r.InstalledByUserID)
	return r, err
}

// ── Stacks ────────────────────────────────────────────────────────────────

func (d *DB) CreateStack(ctx context.Context, s Stack) (Stack, error) {
	const q = `INSERT INTO stacks (user_id, repo_id, name, trunk_branch) VALUES ($1,$2,$3,$4) RETURNING id, created_at`
	if err := d.Pool.QueryRow(ctx, q, s.UserID, s.RepoID, s.Name, s.TrunkBranch).Scan(&s.ID, &s.CreatedAt); err != nil {
		return Stack{}, err
	}
	return s, nil
}

func (d *DB) ListStacks(ctx context.Context, userID, repoID int64) ([]Stack, error) {
	rows, err := d.Pool.Query(ctx, `SELECT id, user_id, repo_id, COALESCE(name,''), trunk_branch, created_at FROM stacks WHERE user_id=$1 AND repo_id=$2 ORDER BY id DESC`, userID, repoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Stack
	for rows.Next() {
		var s Stack
		if err := rows.Scan(&s.ID, &s.UserID, &s.RepoID, &s.Name, &s.TrunkBranch, &s.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (d *DB) GetStackByBranch(ctx context.Context, userID, repoID int64, branch string) (Stack, []StackBranch, error) {
	const findStack = `
SELECT s.id, s.user_id, s.repo_id, COALESCE(s.name,''), s.trunk_branch, s.created_at
FROM stacks s
JOIN stack_branches sb ON sb.stack_id = s.id
WHERE s.user_id=$1 AND s.repo_id=$2 AND sb.name=$3
LIMIT 1`
	var s Stack
	if err := d.Pool.QueryRow(ctx, findStack, userID, repoID, branch).Scan(&s.ID, &s.UserID, &s.RepoID, &s.Name, &s.TrunkBranch, &s.CreatedAt); err != nil {
		return Stack{}, nil, err
	}
	branches, err := d.ListStackBranches(ctx, s.ID)
	return s, branches, err
}

func (d *DB) ListStackBranches(ctx context.Context, stackID int64) ([]StackBranch, error) {
	rows, err := d.Pool.Query(ctx, `SELECT id, stack_id, name, parent_id, position, COALESCE(head_sha,''), pr_id FROM stack_branches WHERE stack_id=$1 ORDER BY position ASC, id ASC`, stackID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []StackBranch
	for rows.Next() {
		var b StackBranch
		if err := rows.Scan(&b.ID, &b.StackID, &b.Name, &b.ParentID, &b.Position, &b.HeadSHA, &b.PRID); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (d *DB) AddStackBranch(ctx context.Context, b StackBranch) (StackBranch, error) {
	const q = `INSERT INTO stack_branches (stack_id, name, parent_id, position, head_sha) VALUES ($1,$2,$3,$4,$5) RETURNING id`
	if err := d.Pool.QueryRow(ctx, q, b.StackID, b.Name, b.ParentID, b.Position, b.HeadSHA).Scan(&b.ID); err != nil {
		return StackBranch{}, err
	}
	return b, nil
}

func (d *DB) UpdateStackBranchParent(ctx context.Context, id int64, parentID *int64, position int) error {
	_, err := d.Pool.Exec(ctx, `UPDATE stack_branches SET parent_id=$1, position=$2, updated_at=NOW() WHERE id=$3`, parentID, position, id)
	return err
}

func (d *DB) DeleteStackBranch(ctx context.Context, id int64) error {
	_, err := d.Pool.Exec(ctx, `DELETE FROM stack_branches WHERE id=$1`, id)
	return err
}

// ── Pull requests ─────────────────────────────────────────────────────────

func (d *DB) UpsertPullRequest(ctx context.Context, pr PullRequest) (PullRequest, error) {
	const q = `
INSERT INTO pull_requests
  (repo_id, number, title, body, state, merged, draft, author_login, author_avatar_url,
   head_branch, base_branch, head_sha, mergeable_state, ci_state,
   additions, deletions, comments, html_url, updated_at_gh, synced_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,NOW())
ON CONFLICT (repo_id, number) DO UPDATE SET
  title=EXCLUDED.title,
  body=EXCLUDED.body,
  state=EXCLUDED.state,
  merged=EXCLUDED.merged,
  draft=EXCLUDED.draft,
  author_login=EXCLUDED.author_login,
  author_avatar_url=EXCLUDED.author_avatar_url,
  head_branch=EXCLUDED.head_branch,
  base_branch=EXCLUDED.base_branch,
  head_sha=EXCLUDED.head_sha,
  mergeable_state=EXCLUDED.mergeable_state,
  ci_state=EXCLUDED.ci_state,
  additions=EXCLUDED.additions,
  deletions=EXCLUDED.deletions,
  comments=EXCLUDED.comments,
  html_url=EXCLUDED.html_url,
  updated_at_gh=EXCLUDED.updated_at_gh,
  synced_at=NOW()
RETURNING id`
	if err := d.Pool.QueryRow(ctx, q,
		pr.RepoID, pr.Number, pr.Title, pr.Body, pr.State, pr.Merged, pr.Draft, pr.AuthorLogin, pr.AuthorAvatarURL,
		pr.HeadBranch, pr.BaseBranch, pr.HeadSHA, pr.MergeableState, pr.CIState,
		pr.Additions, pr.Deletions, pr.Comments, pr.HTMLURL, pr.UpdatedAtGH,
	).Scan(&pr.ID); err != nil {
		return PullRequest{}, err
	}
	return pr, nil
}

func (d *DB) GetPullRequestByID(ctx context.Context, id int64) (PullRequest, error) {
	const q = `SELECT id, repo_id, number, title, COALESCE(body,''), state, merged, draft, author_login, COALESCE(author_avatar_url,''), head_branch, base_branch, COALESCE(head_sha,''), COALESCE(mergeable_state,''), COALESCE(ci_state,''), additions, deletions, comments, html_url, updated_at_gh, synced_at FROM pull_requests WHERE id=$1`
	var pr PullRequest
	err := d.Pool.QueryRow(ctx, q, id).Scan(
		&pr.ID, &pr.RepoID, &pr.Number, &pr.Title, &pr.Body, &pr.State, &pr.Merged, &pr.Draft, &pr.AuthorLogin, &pr.AuthorAvatarURL,
		&pr.HeadBranch, &pr.BaseBranch, &pr.HeadSHA, &pr.MergeableState, &pr.CIState,
		&pr.Additions, &pr.Deletions, &pr.Comments, &pr.HTMLURL, &pr.UpdatedAtGH, &pr.SyncedAt,
	)
	return pr, err
}

func (d *DB) GetPullRequest(ctx context.Context, repoID int64, number int) (PullRequest, error) {
	const q = `SELECT id, repo_id, number, title, COALESCE(body,''), state, merged, draft, author_login, COALESCE(author_avatar_url,''), head_branch, base_branch, COALESCE(head_sha,''), COALESCE(mergeable_state,''), COALESCE(ci_state,''), additions, deletions, comments, html_url, updated_at_gh, synced_at FROM pull_requests WHERE repo_id=$1 AND number=$2`
	var pr PullRequest
	err := d.Pool.QueryRow(ctx, q, repoID, number).Scan(
		&pr.ID, &pr.RepoID, &pr.Number, &pr.Title, &pr.Body, &pr.State, &pr.Merged, &pr.Draft, &pr.AuthorLogin, &pr.AuthorAvatarURL,
		&pr.HeadBranch, &pr.BaseBranch, &pr.HeadSHA, &pr.MergeableState, &pr.CIState,
		&pr.Additions, &pr.Deletions, &pr.Comments, &pr.HTMLURL, &pr.UpdatedAtGH, &pr.SyncedAt,
	)
	return pr, err
}

func (d *DB) ListPullRequests(ctx context.Context, repoID int64, state, author string) ([]PullRequest, error) {
	args := []any{repoID}
	q := `SELECT id, repo_id, number, title, COALESCE(body,''), state, merged, draft, author_login, COALESCE(author_avatar_url,''), head_branch, base_branch, COALESCE(head_sha,''), COALESCE(mergeable_state,''), COALESCE(ci_state,''), additions, deletions, comments, html_url, updated_at_gh, synced_at FROM pull_requests WHERE repo_id=$1`
	if state != "" {
		args = append(args, state)
		q += " AND state=$2"
	}
	if author != "" {
		args = append(args, author)
		q += " AND author_login=$" + itoa(len(args))
	}
	q += " ORDER BY updated_at_gh DESC NULLS LAST"
	rows, err := d.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PullRequest
	for rows.Next() {
		var pr PullRequest
		if err := rows.Scan(&pr.ID, &pr.RepoID, &pr.Number, &pr.Title, &pr.Body, &pr.State, &pr.Merged, &pr.Draft, &pr.AuthorLogin, &pr.AuthorAvatarURL,
			&pr.HeadBranch, &pr.BaseBranch, &pr.HeadSHA, &pr.MergeableState, &pr.CIState,
			&pr.Additions, &pr.Deletions, &pr.Comments, &pr.HTMLURL, &pr.UpdatedAtGH, &pr.SyncedAt); err != nil {
			return nil, err
		}
		out = append(out, pr)
	}
	return out, rows.Err()
}

// ── Merge queue ───────────────────────────────────────────────────────────

func (d *DB) EnqueueMerge(ctx context.Context, repoID, prID, userID int64) (MergeQueueEntry, error) {
	const q = `
INSERT INTO merge_queue_entries (repo_id, pr_id, user_id, position)
VALUES ($1, $2, $3, COALESCE((SELECT MAX(position)+1 FROM merge_queue_entries WHERE repo_id=$1), 1))
ON CONFLICT (repo_id, pr_id) DO UPDATE SET state='queued', last_error=NULL, last_event_at=NOW()
RETURNING id, state, position, attempts, COALESCE(last_error,''), last_event_at, enqueued_at, merged_at`
	var e MergeQueueEntry
	e.RepoID, e.PRID, e.UserID = repoID, prID, userID
	err := d.Pool.QueryRow(ctx, q, repoID, prID, userID).Scan(
		&e.ID, &e.State, &e.Position, &e.Attempts, &e.LastError, &e.LastEventAt, &e.EnqueuedAt, &e.MergedAt,
	)
	return e, err
}

func (d *DB) ListMergeQueue(ctx context.Context, repoID int64) ([]MergeQueueEntry, error) {
	rows, err := d.Pool.Query(ctx, `SELECT id, repo_id, pr_id, user_id, state::text, position, attempts, COALESCE(last_error,''), last_event_at, enqueued_at, merged_at FROM merge_queue_entries WHERE repo_id=$1 ORDER BY position ASC`, repoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MergeQueueEntry
	for rows.Next() {
		var e MergeQueueEntry
		if err := rows.Scan(&e.ID, &e.RepoID, &e.PRID, &e.UserID, &e.State, &e.Position, &e.Attempts, &e.LastError, &e.LastEventAt, &e.EnqueuedAt, &e.MergedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (d *DB) NextMergeQueueEntry(ctx context.Context, repoID int64) (MergeQueueEntry, error) {
	const q = `SELECT id, repo_id, pr_id, user_id, state::text, position, attempts, COALESCE(last_error,''), last_event_at, enqueued_at, merged_at
FROM merge_queue_entries
WHERE repo_id=$1 AND state IN ('queued','integrating','awaiting_ci','merging')
ORDER BY position ASC LIMIT 1`
	var e MergeQueueEntry
	err := d.Pool.QueryRow(ctx, q, repoID).Scan(&e.ID, &e.RepoID, &e.PRID, &e.UserID, &e.State, &e.Position, &e.Attempts, &e.LastError, &e.LastEventAt, &e.EnqueuedAt, &e.MergedAt)
	return e, err
}

func (d *DB) UpdateMergeQueueState(ctx context.Context, id int64, state, lastError string, attempts int, mergedAt *time.Time) error {
	_, err := d.Pool.Exec(ctx,
		`UPDATE merge_queue_entries SET state=$2::merge_queue_state, last_error=NULLIF($3,''), attempts=$4, last_event_at=NOW(), merged_at=$5 WHERE id=$1`,
		id, state, lastError, attempts, mergedAt)
	return err
}

func (d *DB) CancelMergeQueueEntry(ctx context.Context, repoID, id int64) error {
	_, err := d.Pool.Exec(ctx, `UPDATE merge_queue_entries SET state='cancelled', last_event_at=NOW() WHERE id=$1 AND repo_id=$2`, id, repoID)
	return err
}

// ── Webhook events ────────────────────────────────────────────────────────

func (d *DB) RecordWebhookEvent(ctx context.Context, deliveryID, eventType string, repoID *int64, payload []byte) (bool, error) {
	const q = `INSERT INTO webhook_events (delivery_id, repo_id, event_type, payload) VALUES ($1,$2,$3,$4) ON CONFLICT (delivery_id) DO NOTHING RETURNING id`
	var id int64
	err := d.Pool.QueryRow(ctx, q, deliveryID, repoID, eventType, payload).Scan(&id)
	if err == pgx.ErrNoRows {
		return false, nil // already processed
	}
	return err == nil, err
}

// itoa is a tiny inlined Itoa helper to keep the dynamic SQL in
// ListPullRequests readable without pulling in strconv from inside a query
// builder. Returns base-10 representation.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
