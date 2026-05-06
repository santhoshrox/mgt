// Package mergequeue contains the FIFO merge worker. One per repo. State
// machine: queued -> awaiting_ci -> merging -> merged. Failures move to the
// `failed` state with the GitHub error stored in last_error.
package mergequeue

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/santhosh/mgt-be/internal/crypto"
	"github.com/santhosh/mgt-be/internal/db"
	"github.com/santhosh/mgt-be/internal/github"
)

const maxAttempts = 5

type Worker struct {
	db     *db.DB
	sealer *crypto.Sealer

	notify   chan int64
	pollEvery time.Duration

	mu      sync.Mutex
	running bool
}

func New(d *db.DB, sealer *crypto.Sealer, pollSeconds int) *Worker {
	if pollSeconds <= 0 {
		pollSeconds = 30
	}
	return &Worker{
		db:        d,
		sealer:    sealer,
		notify:    make(chan int64, 64),
		pollEvery: time.Duration(pollSeconds) * time.Second,
	}
}

// Notify wakes the worker for the given repo (called from the API after
// enqueue/cancel and from the webhook handler after a state-changing event).
func (w *Worker) Notify(repoID int64) {
	select {
	case w.notify <- repoID:
	default:
	}
}

// Run pumps work until ctx is done. It blocks; call as a goroutine.
func (w *Worker) Run(ctx context.Context) {
	t := time.NewTicker(w.pollEvery)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			w.scanAll(ctx)
		case repoID := <-w.notify:
			w.processRepo(ctx, repoID)
		}
	}
}

func (w *Worker) scanAll(ctx context.Context) {
	repos, err := w.db.ListRepositories(ctx)
	if err != nil {
		slog.Warn("queue scan: list repos", "err", err)
		return
	}
	for _, r := range repos {
		w.processRepo(ctx, r.ID)
	}
}

func (w *Worker) processRepo(ctx context.Context, repoID int64) {
	repo, err := w.db.GetRepository(ctx, repoID)
	if err != nil {
		return
	}
	entry, err := w.db.NextMergeQueueEntry(ctx, repoID)
	if err != nil {
		return // empty queue: pgx ErrNoRows
	}

	user, err := w.db.GetUserByID(ctx, entry.UserID)
	if err != nil {
		w.fail(ctx, entry, "load user: "+err.Error())
		return
	}
	tok, err := w.sealer.Open(user.EncryptedOAuthToken)
	if err != nil {
		w.fail(ctx, entry, "decrypt token: "+err.Error())
		return
	}
	gh := github.New(tok)

	pr, err := w.db.GetPullRequestByID(ctx, entry.PRID)
	if err != nil {
		w.fail(ctx, entry, "load pr: "+err.Error())
		return
	}

	switch entry.State {
	case "queued":
		// Move to awaiting_ci and check status.
		if err := w.db.UpdateMergeQueueState(ctx, entry.ID, "awaiting_ci", "", entry.Attempts+1, nil); err != nil {
			slog.Warn("queue: update state", "err", err)
			return
		}
		fallthrough
	case "awaiting_ci":
		state, err := gh.CombinedStatus(repo.Owner, repo.Name, pr.HeadSHA)
		if err != nil {
			w.bumpAttemptsOrFail(ctx, entry, err)
			return
		}
		switch state {
		case "success":
			_ = w.db.UpdateMergeQueueState(ctx, entry.ID, "merging", "", entry.Attempts, nil)
			w.merge(ctx, entry, repo, gh, pr)
		case "failure", "error":
			w.fail(ctx, entry, "CI failed")
		case "pending", "":
			// Stay in awaiting_ci; webhook or next tick will retry.
			return
		}
	case "merging":
		w.merge(ctx, entry, repo, gh, pr)
	}
}

func (w *Worker) merge(ctx context.Context, entry db.MergeQueueEntry, repo db.Repository, gh *github.Client, pr db.PullRequest) {
	res, err := gh.MergePR(repo.Owner, repo.Name, pr.Number, "squash")
	if err != nil {
		w.bumpAttemptsOrFail(ctx, entry, err)
		return
	}
	if !res.Merged {
		w.fail(ctx, entry, "merge rejected: "+res.Message)
		return
	}
	now := time.Now()
	_ = w.db.UpdateMergeQueueState(ctx, entry.ID, "merged", "", entry.Attempts, &now)
}

func (w *Worker) bumpAttemptsOrFail(ctx context.Context, e db.MergeQueueEntry, err error) {
	msg := err.Error()
	// Detect transient errors via substring (light-touch).
	transient := strings.Contains(msg, "5") && strings.Contains(msg, "github") // 5xx
	attempts := e.Attempts + 1
	if !transient && attempts >= maxAttempts {
		w.fail(ctx, e, msg)
		return
	}
	_ = w.db.UpdateMergeQueueState(ctx, e.ID, "awaiting_ci", msg, attempts, nil)
}

func (w *Worker) fail(ctx context.Context, e db.MergeQueueEntry, msg string) {
	if msg == "" {
		msg = "unknown error"
	}
	if err := w.db.UpdateMergeQueueState(ctx, e.ID, "failed", msg, e.Attempts, nil); err != nil {
		slog.Warn("queue: mark failed", "err", err)
	}
}

// silence unused-import in case ctx imports are dropped during refactors
var _ = errors.New
