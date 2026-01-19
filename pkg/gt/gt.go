package gt

import (
	"os"
	"os/exec"
	"strings"
	"syscall"
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

// GetTrunk returns the trunk branch name.
func GetTrunk() (string, error) {
	// Try to get from charcoal if possible, otherwise fallback to main/master
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

	// Fallback
	for _, b := range []string{"main", "master", "trunk"} {
		if err := exec.Command("git", "rev-parse", "--verify", b).Run(); err == nil {
			return b, nil
		}
	}
	return "main", nil
}

// GetStacks returns a list of local branch stacks (not trunk).
func GetStacks() ([]string, error) {
	// gt log short gives a nice view. We might want to parse it or just use gt branch checkout's native selector.
	// For "selector" from trunk, 'gt branch checkout' is likely what the user wants if it shows local stacks.
	return nil, nil
}
