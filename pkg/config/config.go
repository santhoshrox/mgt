package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// ConfigFilename is the name of the config file in the user's home directory.
const ConfigFilename = ".mgt"

// ConfigPath returns the full path to the config file (~/.mgt). Empty string on error.
func ConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ConfigFilename)
}

// BranchPrefix returns the configured branch name prefix for new stacks/branches.
// For example, "santhosh/" so that "mgt create my-feature" creates "santhosh/my-feature".
// Read from MGT_BRANCH_PREFIX (env), then ~/.mgt (branch_prefix=...). Returns empty string if unset.
func BranchPrefix() string {
	s := strings.TrimSpace(os.Getenv("MGT_BRANCH_PREFIX"))
	if s == "" {
		s = readBranchPrefixFromFile()
	}
	// Ensure prefix ends with / when non-empty so "santhosh" becomes "santhosh/"
	if s != "" && !strings.HasSuffix(s, "/") {
		s = s + "/"
	}
	return s
}

// SetBranchPrefix writes branch_prefix to ~/.mgt, creating the file if needed.
// Pass "" to set no prefix. Existing branch_prefix line is updated; other lines are preserved.
func SetBranchPrefix(value string) error {
	path := ConfigPath()
	if path == "" {
		return os.ErrNotExist
	}
	lines, err := readConfigLines(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	replaced := false
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "branch_prefix=") {
			lines[i] = "branch_prefix=" + value
			replaced = true
			break
		}
	}
	if !replaced {
		lines = append(lines, "branch_prefix="+value)
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0600)
}

func readBranchPrefixFromFile() string {
	path := ConfigPath()
	if path == "" {
		return ""
	}
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "branch_prefix=") {
			return strings.TrimSpace(strings.TrimPrefix(line, "branch_prefix="))
		}
	}
	return ""
}

// readConfigLines returns all lines from the config file, or nil if file does not exist.
func readConfigLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}
