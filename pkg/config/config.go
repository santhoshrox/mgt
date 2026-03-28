package config

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const ConfigFilename = ".mgt"

// ConfigPath returns the full path to the user-level config file (~/.mgt).
func ConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ConfigFilename)
}

// GitRootPath returns the root of the current git repository.
func GitRootPath() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// RepoConfigPath returns the path to the per-repo config file (<git-root>/.mgt).
func RepoConfigPath() string {
	root, err := GitRootPath()
	if err != nil {
		return ""
	}
	return filepath.Join(root, ConfigFilename)
}

// BranchPrefix returns the configured branch name prefix for new stacks/branches.
// Read from MGT_BRANCH_PREFIX (env), then ~/.mgt (branch_prefix=...).
func BranchPrefix() string {
	s := strings.TrimSpace(os.Getenv("MGT_BRANCH_PREFIX"))
	if s == "" {
		s = readValueFromFile(ConfigPath(), "branch_prefix")
	}
	if s != "" && !strings.HasSuffix(s, "/") {
		s = s + "/"
	}
	return s
}

// SetBranchPrefix writes branch_prefix to ~/.mgt, creating the file if needed.
func SetBranchPrefix(value string) error {
	path := ConfigPath()
	if path == "" {
		return os.ErrNotExist
	}
	return setValueInFile(path, "branch_prefix", value)
}

// LLMKey returns the configured API key for the LLM provider.
// Reads: LLM_API_KEY env → OPENAI_API_KEY env → llm_key in ~/.mgt → openai_key in ~/.mgt.
func LLMKey() string {
	for _, env := range []string{"LLM_API_KEY", "OPENAI_API_KEY"} {
		if s := strings.TrimSpace(os.Getenv(env)); s != "" {
			return s
		}
	}
	if s := readValueFromFile(ConfigPath(), "llm_key"); s != "" {
		return s
	}
	return readValueFromFile(ConfigPath(), "openai_key")
}

// LLMBaseURL returns the base URL for the LLM API.
// Reads: LLM_BASE_URL env → llm_base_url in ~/.mgt. Defaults to OpenAI.
func LLMBaseURL() string {
	if s := strings.TrimSpace(os.Getenv("LLM_BASE_URL")); s != "" {
		return strings.TrimRight(s, "/")
	}
	if s := readValueFromFile(ConfigPath(), "llm_base_url"); s != "" {
		return strings.TrimRight(s, "/")
	}
	return "https://api.openai.com/v1"
}

// LLMModel returns the model name for the LLM provider.
// Reads: LLM_MODEL env → llm_model in ~/.mgt. Defaults to gpt-4o-mini.
func LLMModel() string {
	if s := strings.TrimSpace(os.Getenv("LLM_MODEL")); s != "" {
		return s
	}
	if s := readValueFromFile(ConfigPath(), "llm_model"); s != "" {
		return s
	}
	return "gpt-4o-mini"
}

// SetLLMValue writes an llm config key to ~/.mgt.
func SetLLMValue(key, value string) error {
	path := ConfigPath()
	if path == "" {
		return os.ErrNotExist
	}
	return setValueInFile(path, key, value)
}

// Trunk returns the configured trunk branch for the current repo.
// Reads from <git-root>/.mgt (trunk=...). Returns empty string if unset.
func Trunk() string {
	return readValueFromFile(RepoConfigPath(), "trunk")
}

// Remote returns the configured default remote for the current repo.
// Reads from <git-root>/.mgt (remote=...). Returns empty string if unset.
func Remote() string {
	return readValueFromFile(RepoConfigPath(), "remote")
}

// SetRepoValue writes a key=value pair to the per-repo config (<git-root>/.mgt).
func SetRepoValue(key, value string) error {
	path := RepoConfigPath()
	if path == "" {
		return fmt.Errorf("not in a git repository")
	}
	return setValueInFile(path, key, value)
}

func readValueFromFile(path, key string) string {
	if path == "" {
		return ""
	}
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	prefix := key + "="
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix))
		}
	}
	return ""
}

func setValueInFile(path, key, value string) error {
	lines, err := readConfigLines(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	prefix := key + "="
	replaced := false
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), prefix) {
			lines[i] = prefix + value
			replaced = true
			break
		}
	}
	if !replaced {
		lines = append(lines, prefix+value)
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0600)
}

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
