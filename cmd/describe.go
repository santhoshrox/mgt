package cmd

import (
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
	Long:  "Reads the repo's GitHub PR template (or a built-in default), inspects the branch diff and commits, and uses OpenAI to fill it in. Applies the result to the open PR via `gh pr edit`.",
	Run: func(cmd *cobra.Command, args []string) {
		trunk, err := gt.GetTrunk()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error detecting trunk: %v\n", err)
			os.Exit(1)
		}

		diff, _ := exec.Command("git", "diff", trunk+"...HEAD").Output()
		commits, _ := exec.Command("git", "log", trunk+"..HEAD", "--oneline").Output()

		if len(diff) == 0 {
			fmt.Fprintln(os.Stderr, "No changes found on this branch.")
			os.Exit(1)
		}

		template := findPRTemplate()

		fmt.Fprint(os.Stderr, "Generating PR description...")
		body, err := ai.FillPRBody(template, string(diff), string(commits))
		if err != nil {
			fmt.Fprintf(os.Stderr, " failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, " done.")

		ghCmd := exec.Command("gh", "pr", "edit", "--body-file", "-")
		ghCmd.Stdin = strings.NewReader(body)
		ghCmd.Stdout = os.Stdout
		ghCmd.Stderr = os.Stderr
		if err := ghCmd.Run(); err != nil {
			fmt.Fprintln(os.Stderr, "Could not update PR body via gh, copying to clipboard instead.")
			if copyToClipboard(body) {
				fmt.Fprintln(os.Stderr, "PR description copied to clipboard.")
			} else {
				fmt.Fprintln(os.Stderr, "---")
				fmt.Fprintln(os.Stderr, body)
				fmt.Fprintln(os.Stderr, "---")
			}
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, "PR description updated.")
	},
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
			fmt.Fprintln(os.Stderr, "Found PR template:", c)
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
