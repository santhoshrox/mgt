package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/santhosh/mgt/pkg/ai"
	"github.com/santhosh/mgt/pkg/config"
	"github.com/santhosh/mgt/pkg/gt"
	"github.com/spf13/cobra"
)

const defaultPRTemplate = `## Summary

<!-- What does this PR do and why? -->

## Changes

<!-- List the key changes -->

## Test Plan

<!-- How was this tested? -->
`

var describeCmd = &cobra.Command{
	Use:   "describe",
	Short: "Generate an AI PR description for the current branch",
	Long:  "Reads the repo's GitHub PR template (or a built-in default), inspects the branch diff and commits, and calls the configured LLM to fill it in. Applies the result to the open PR via `gh pr edit`.",
	Run: func(cmd *cobra.Command, args []string) {
		current, err := gt.GetCurrentBranch()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting current branch: %v\n", err)
			os.Exit(1)
		}
		parent := branchParent(current)
		template := findPRTemplate()

		fmt.Fprintf(os.Stderr, "Generating PR description for %s...", current)
		body, err := generateDescription(current, parent, template)
		if err != nil {
			fmt.Fprintf(os.Stderr, " failed: %v\n", err)
			os.Exit(1)
		}
		if body == "" {
			fmt.Fprintln(os.Stderr, " no changes found.")
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, " done.")
		applyPRBody(current, body)
	},
}

// llmConfigured returns true if the user has set up an LLM provider
// (either a key for cloud providers, or a custom base URL for local ones like Ollama).
func llmConfigured() bool {
	if config.LLMKey() != "" {
		return true
	}
	return config.LLMBaseURL() != "https://api.openai.com/v1"
}

// promptForAI asks the user whether to generate AI PR descriptions.
func promptForAI() bool {
	if !llmConfigured() {
		return false
	}
	fmt.Fprint(os.Stderr, "Generate AI PR description? [y/N] ")
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
		return answer == "y" || answer == "yes"
	}
	return false
}

func generateDescription(branch, parent, template string) (string, error) {
	diff, _ := exec.Command("git", "diff", parent+"..."+branch).Output()
	commits, _ := exec.Command("git", "log", parent+".."+branch, "--oneline").Output()
	if len(diff) == 0 {
		return "", nil
	}
	return ai.FillPRBody(template, string(diff), string(commits))
}

func applyPRBody(branch, body string) {
	ghCmd := exec.Command("gh", "pr", "edit", branch, "--body-file", "-")
	ghCmd.Stdin = strings.NewReader(body)
	ghCmd.Stdout = os.Stdout
	ghCmd.Stderr = os.Stderr
	if err := ghCmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Could not update PR body for %s via gh, copying to clipboard instead.\n", branch)
		if copyToClipboard(body) {
			fmt.Fprintln(os.Stderr, "PR description copied to clipboard.")
		} else {
			fmt.Fprintln(os.Stderr, "---")
			fmt.Fprintln(os.Stderr, body)
			fmt.Fprintln(os.Stderr, "---")
		}
		return
	}
	fmt.Fprintf(os.Stderr, "PR description updated for %s.\n", branch)
}

func branchParent(branch string) string {
	trunk, err := gt.GetTrunk()
	if err != nil {
		return "main"
	}
	branches, err := gt.GetStackBranches()
	if err != nil {
		return trunk
	}
	for i, b := range branches {
		if b == branch && i > 0 {
			return branches[i-1]
		}
	}
	return trunk
}

func findPRTemplate() string {
	root, err := config.GitRootPath()
	if err != nil {
		return defaultPRTemplate
	}
	candidates := []string{
		".github/PULL_REQUEST_TEMPLATE.md",
		".github/pull_request_template.md",
		"PULL_REQUEST_TEMPLATE.md",
		"pull_request_template.md",
		"docs/pull_request_template.md",
	}
	for _, c := range candidates {
		data, err := os.ReadFile(filepath.Join(root, c))
		if err == nil {
			if t := strings.TrimSpace(string(data)); t != "" {
				return t
			}
		}
	}
	return defaultPRTemplate
}

func copyToClipboard(text string) bool {
	cmd := exec.Command("pbcopy")
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run() == nil
}

func init() {
	rootCmd.AddCommand(describeCmd)
}
