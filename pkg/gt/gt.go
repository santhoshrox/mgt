package gt

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"syscall"

	"github.com/santhosh/mgt/pkg/config"
)

// Run executes the gt command as a subprocess.
func Run(args ...string) error {
	cmd := exec.Command("gt", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

// Exec replaces the current process with the gt command.
// This is preferred for interactive commands to ensure perfect TTY handling.
func Exec(args ...string) error {
	gtPath, err := exec.LookPath("gt")
	if err != nil {
		return err
	}

	// syscall.Exec requires the command name as the first argument in the slice
	execArgs := append([]string{"gt"}, args...)
	return syscall.Exec(gtPath, execArgs, os.Environ())
}

// GetCurrentBranch returns the name of the current git branch.
func GetCurrentBranch() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// RenameCurrentBranch renames the current branch to newName using gt,
// which updates the branch metadata refs that track parent-child relationships.
func RenameCurrentBranch(newName string) error {
	cmd := exec.Command("gt", "branch", "rename", newName, "--force")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// EnsureDefaultRemote sets git's checkout.defaultRemote to the configured remote
// so that commands like `git switch main` are unambiguous when multiple remotes
// have a branch with the same name.
func EnsureDefaultRemote() {
	remote := config.Remote()
	if remote == "" {
		return
	}
	exec.Command("git", "config", "checkout.defaultRemote", remote).Run()
}

// GetTrunk returns the trunk branch name.
func GetTrunk() (string, error) {
	if t := config.Trunk(); t != "" {
		return t, nil
	}

	out, err := exec.Command("gt", "repo", "info").CombinedOutput()
	if err == nil {
		lines := strings.Split(string(out), "\n")
		for _, line := range lines {
			if strings.Contains(line, "Trunk:") {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					return parts[1], nil
				}
			}
		}
	}

	for _, b := range []string{"main", "master", "trunk"} {
		if err := exec.Command("git", "rev-parse", "--verify", b).Run(); err == nil {
			return b, nil
		}
	}
	return "main", nil
}

// GetStackBranches returns the branches in the current stack, ordered base→tip (trunk excluded).
// It parses `gt log short` which lists branches tip→base.
func GetStackBranches() ([]string, error) {
	trunk, err := GetTrunk()
	if err != nil {
		return nil, err
	}
	out, err := exec.Command("gt", "log", "short").Output()
	if err != nil {
		return nil, fmt.Errorf("gt log short: %w", err)
	}
	return parseStackBranches(string(out), trunk), nil
}

var ansiEscape = regexp.MustCompile(`\x1b\[[0-9;]*[mGKHFJA-Za-z]`)

func stripANSI(s string) string {
	return ansiEscape.ReplaceAllString(s, "")
}

// parseStackBranches extracts branch names from `gt log short` output.
// gt log short prints tip first; we reverse to return base→tip order.
func parseStackBranches(output, trunk string) []string {
	var branches []string
	for _, line := range strings.Split(output, "\n") {
		clean := stripANSI(line)
		fields := strings.Fields(clean)
		// Find the token containing a branch indicator symbol (◉ ◯ ◎ ●)
		for i, f := range fields {
			if strings.ContainsAny(f, "◉◯◎●") && i+1 < len(fields) {
				name := fields[i+1]
				if name != trunk && name != "" {
					branches = append(branches, name)
				}
				break
			}
		}
	}
	// Reverse: gt log short lists tip first, we want base→tip
	for i, j := 0, len(branches)-1; i < j; i, j = i+1, j-1 {
		branches[i], branches[j] = branches[j], branches[i]
	}
	return branches
}

// ExecuteReorder performs the branch reorder given newOrder (base→tip).
// It checks out each branch and runs `gt upstack onto <parent>` to place it correctly,
// then returns to returnBranch.
func ExecuteReorder(trunk string, newOrder []string, returnBranch string) error {
	for i, branch := range newOrder {
		parent := trunk
		if i > 0 {
			parent = newOrder[i-1]
		}
		checkout := exec.Command("git", "checkout", "-q", branch)
		checkout.Stdout = os.Stdout
		checkout.Stderr = os.Stderr
		if err := checkout.Run(); err != nil {
			return fmt.Errorf("checkout %s: %w", branch, err)
		}
		if err := Run("upstack", "onto", parent); err != nil {
			return fmt.Errorf("upstack onto %s for %s: %w", parent, branch, err)
		}
	}
	checkout := exec.Command("git", "checkout", "-q", returnBranch)
	checkout.Stdout = os.Stdout
	checkout.Stderr = os.Stderr
	return checkout.Run()
}
