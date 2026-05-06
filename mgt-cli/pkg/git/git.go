// Package git is a tiny shell over the local `git` binary. Commands that
// need an interactive TTY (rebase with conflicts, push prompting for creds)
// inherit our stdio.
package git

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Run executes git with stdout/stderr/stdin attached.
func Run(args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

// Output runs git and returns trimmed stdout.
func Output(args ...string) (string, error) {
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// CurrentBranch returns the checked-out branch name.
func CurrentBranch() (string, error) {
	return Output("rev-parse", "--abbrev-ref", "HEAD")
}

// RootPath returns the absolute path to the git working tree.
func RootPath() (string, error) {
	return Output("rev-parse", "--show-toplevel")
}

// RemoteURL returns the URL for `origin` (or "" if missing).
func RemoteURL(remote string) string {
	if remote == "" {
		remote = "origin"
	}
	out, _ := Output("remote", "get-url", remote)
	return out
}

// OwnerRepo parses an https:// or git@ remote URL into its owner and repo
// pair. It returns ("", "") if it can't recognise the URL.
func OwnerRepo(url string) (owner, name string) {
	url = strings.TrimSuffix(url, ".git")
	if strings.HasPrefix(url, "git@") {
		// git@github.com:owner/repo
		if i := strings.Index(url, ":"); i >= 0 {
			parts := strings.SplitN(url[i+1:], "/", 2)
			if len(parts) == 2 {
				return parts[0], parts[1]
			}
		}
		return "", ""
	}
	// https://github.com/owner/repo
	if i := strings.Index(url, "://"); i >= 0 {
		rest := url[i+3:]
		// strip host
		if j := strings.Index(rest, "/"); j >= 0 {
			rest = rest[j+1:]
		}
		parts := strings.SplitN(rest, "/", 2)
		if len(parts) == 2 {
			return parts[0], parts[1]
		}
	}
	return "", ""
}

// CheckoutQuiet runs `git checkout -q <branch>`.
func CheckoutQuiet(branch string) error {
	cmd := exec.Command("git", "checkout", "-q", branch)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// CreateBranch runs `git checkout -b <name>` from the current HEAD.
func CreateBranch(name string) error {
	return Run("checkout", "-b", name)
}

// Push pushes the named branch to the given remote, setting upstream.
func Push(remote, branch string) error {
	return Run("push", "-u", remote, branch)
}

// RebaseOnto rebases `branch` onto `newBase`, taking the commits from
// `oldBase` (the previous parent) when newBase != oldBase.
//
//	git rebase --onto <newBase> <oldBase> <branch>
//
// If oldBase is empty, falls back to plain `git rebase <newBase> <branch>`.
func RebaseOnto(branch, oldBase, newBase string) error {
	if oldBase == "" {
		return Run("rebase", newBase, branch)
	}
	return Run("rebase", "--onto", newBase, oldBase, branch)
}

// DeleteBranch is `git branch -D <name>`.
func DeleteBranch(name string) error {
	return Run("branch", "-D", name)
}

// MergeBase returns `git merge-base a b` (used to find oldBase before a rebase).
func MergeBase(a, b string) (string, error) {
	return Output("merge-base", a, b)
}

// FetchAll runs `git fetch --all --prune`.
func FetchAll() error {
	return Run("fetch", "--all", "--prune")
}

// EnsureRepo aborts with a friendly message if we're not inside a git repo.
func EnsureRepo() error {
	if _, err := RootPath(); err != nil {
		return fmt.Errorf("not inside a git repository")
	}
	return nil
}
