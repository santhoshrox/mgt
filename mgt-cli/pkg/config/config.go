// Package config holds the (now minimal) CLI configuration.
//
// User-level settings live in ~/.mgt/config (key=value); the bearer token
// lives in ~/.mgt/credentials (mode 0600). Everything else (LLM, trunk,
// remote, …) is now owned by mgt-be.
package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	dirName          = ".mgt"
	configFile       = "config"
	credentialsFile  = "credentials"
	legacyConfigFile = "" // legacy ~/.mgt was a flat file; handled below
	defaultServerURL = "http://localhost:8080"
	defaultGRPCAddr  = "localhost:9090"
)

// ConfigDir returns ~/.mgt/, creating it if needed (mode 0700).
func ConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, dirName)
	if err := ensureDir(dir); err != nil {
		return "", err
	}
	return dir, nil
}

// ensureDir mkdirs the path with 0700 perms unless it already exists as a
// regular file (legacy ~/.mgt). In that case we move it aside so we can use
// the directory layout.
func ensureDir(dir string) error {
	info, err := os.Stat(dir)
	if err == nil {
		if !info.IsDir() {
			// Legacy: ~/.mgt was a flat ini file. Migrate it.
			if err := migrateLegacy(dir); err != nil {
				return err
			}
			return os.MkdirAll(dir, 0o700)
		}
		return nil
	}
	if !os.IsNotExist(err) {
		return err
	}
	return os.MkdirAll(dir, 0o700)
}

// migrateLegacy reads branch_prefix from a flat ~/.mgt file and prepares to
// rewrite it under ~/.mgt/config. The legacy file is renamed to ~/.mgt.bak.
func migrateLegacy(legacyPath string) error {
	prefix := readKeyFromFile(legacyPath, "branch_prefix")
	if err := os.Rename(legacyPath, legacyPath+".bak"); err != nil {
		return err
	}
	if prefix == "" {
		return nil
	}
	if err := os.MkdirAll(legacyPath, 0o700); err != nil {
		return err
	}
	return writeKeyToFile(filepath.Join(legacyPath, configFile), "branch_prefix", prefix)
}

// ── Public accessors ──────────────────────────────────────────────────────

func ConfigPath() string {
	dir, err := ConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, configFile)
}

func CredentialsPath() string {
	dir, err := ConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, credentialsFile)
}

// ServerURL returns the mgt-be base URL. MGT_SERVER_URL > config file > default.
//
// Used for the rare web-only operations (today none — all CLI traffic is
// gRPC). Kept for the device-flow `verification_url` printout.
func ServerURL() string {
	if v := strings.TrimSpace(os.Getenv("MGT_SERVER_URL")); v != "" {
		return strings.TrimRight(v, "/")
	}
	if v := readKeyFromFile(ConfigPath(), "server_url"); v != "" {
		return strings.TrimRight(v, "/")
	}
	return defaultServerURL
}

// GRPCAddr returns host:port for the mgt-be gRPC endpoint.
// MGT_SERVER_GRPC_ADDR > config file `server_grpc_addr` > default.
func GRPCAddr() string {
	if v := strings.TrimSpace(os.Getenv("MGT_SERVER_GRPC_ADDR")); v != "" {
		return v
	}
	if v := readKeyFromFile(ConfigPath(), "server_grpc_addr"); v != "" {
		return v
	}
	return defaultGRPCAddr
}

// GRPCInsecure controls whether to dial the gRPC server with TLS. Defaults
// to true (insecure) for local self-hosted deployments. Toggle via env
// MGT_SERVER_GRPC_INSECURE=0 or `server_grpc_insecure=0` once you've put
// mgt-be behind a TLS-terminating proxy.
func GRPCInsecure() bool {
	if v := strings.TrimSpace(os.Getenv("MGT_SERVER_GRPC_INSECURE")); v != "" {
		return v != "0" && strings.ToLower(v) != "false"
	}
	if v := readKeyFromFile(ConfigPath(), "server_grpc_insecure"); v != "" {
		return v != "0" && strings.ToLower(v) != "false"
	}
	return true
}

// BranchPrefix returns the prefix prepended to new branch names.
// MGT_BRANCH_PREFIX > config file. A trailing "/" is appended if missing.
func BranchPrefix() string {
	v := strings.TrimSpace(os.Getenv("MGT_BRANCH_PREFIX"))
	if v == "" {
		v = readKeyFromFile(ConfigPath(), "branch_prefix")
	}
	if v != "" && !strings.HasSuffix(v, "/") {
		v += "/"
	}
	return v
}

// Token returns the bearer token from MGT_TOKEN or ~/.mgt/credentials.
func Token() string {
	if v := strings.TrimSpace(os.Getenv("MGT_TOKEN")); v != "" {
		return v
	}
	return strings.TrimSpace(readFile(CredentialsPath()))
}

// ── Setters used by `mgt config set`, `mgt login`, etc. ──────────────────

func SetServerURL(url string) error {
	return writeKeyToFile(ConfigPath(), "server_url", url)
}

func SetGRPCAddr(addr string) error {
	return writeKeyToFile(ConfigPath(), "server_grpc_addr", addr)
}

func SetBranchPrefix(prefix string) error {
	return writeKeyToFile(ConfigPath(), "branch_prefix", strings.TrimSuffix(prefix, "/"))
}

func SetToken(token string) error {
	path := CredentialsPath()
	if path == "" {
		return fmt.Errorf("could not determine credentials path")
	}
	if token == "" {
		_ = os.Remove(path)
		return nil
	}
	return os.WriteFile(path, []byte(token+"\n"), 0o600)
}

// ── helpers ───────────────────────────────────────────────────────────────

func readFile(path string) string {
	if path == "" {
		return ""
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(b)
}

func readKeyFromFile(path, key string) string {
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

func writeKeyToFile(path, key, value string) error {
	if path == "" {
		return fmt.Errorf("config path unavailable")
	}
	lines, err := readLines(path)
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
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}

func readLines(path string) ([]string, error) {
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
