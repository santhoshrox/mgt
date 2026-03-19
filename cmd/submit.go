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

var submitCmd = &cobra.Command{
	Use:                "submit",
	Short:              "Submit the current branch as a pull request",
	Long:               "Submits via `gt branch submit`. If an OpenAI key is configured, generates a filled PR description (from the repo's GitHub template or a default) and copies it to your clipboard.",
	DisableFlagParsing: true,
	Args:               cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		tryFillPRBody()

		gtArgs := []string{"branch", "submit"}
		gtArgs = append(gtArgs, args...)
		gt.Exec(gtArgs...)
	},
}

func tryFillPRBody() {
	if config.OpenAIKey() == "" {
		return
	}

	template := findPRTemplate()

	trunk, err := gt.GetTrunk()
	if err != nil {
		return
	}

	diff, _ := exec.Command("git", "diff", trunk+"...HEAD").Output()
	commits, _ := exec.Command("git", "log", trunk+"..HEAD", "--oneline").Output()

	if len(diff) == 0 {
		return
	}

	fmt.Fprint(os.Stderr, "Generating PR description...")
	body, err := ai.FillPRBody(template, string(diff), string(commits))
	if err != nil {
		fmt.Fprintf(os.Stderr, " skipped (%v)\n", err)
		return
	}

	if copyToClipboard(body) {
		fmt.Fprintln(os.Stderr, " copied to clipboard!")
	} else {
		fmt.Fprintln(os.Stderr, " done.")
		fmt.Fprintln(os.Stderr, "---")
		fmt.Fprintln(os.Stderr, body)
		fmt.Fprintln(os.Stderr, "---")
	}
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
	rootCmd.AddCommand(submitCmd)
}
