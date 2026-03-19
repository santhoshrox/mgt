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

var submitCmd = &cobra.Command{
	Use:                "submit",
	Short:              "Submit the current branch as a pull request",
	Long:               "Submits via `gt branch submit`. After the PR is created, offers to generate an AI-filled PR description if an OpenAI key is configured.",
	DisableFlagParsing: true,
	Args:               cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		canGenerate := config.OpenAIKey() != ""

		gtArgs := []string{"branch", "submit"}
		gtArgs = append(gtArgs, args...)

		if canGenerate {
			if err := gt.Run(gtArgs...); err != nil {
				os.Exit(1)
			}
			promptAndGeneratePRBody()
		} else {
			gt.Exec(gtArgs...)
		}
	},
}

func promptAndGeneratePRBody() {
	fmt.Fprint(os.Stderr, "Generate AI PR description? [Y/n] ")
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
		if answer == "n" || answer == "no" {
			return
		}
	}

	trunk, err := gt.GetTrunk()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error detecting trunk: %v\n", err)
		return
	}

	diff, _ := exec.Command("git", "diff", trunk+"...HEAD").Output()
	commits, _ := exec.Command("git", "log", trunk+"..HEAD", "--oneline").Output()

	if len(diff) == 0 {
		fmt.Fprintln(os.Stderr, "No diff found, skipping.")
		return
	}

	template := findPRTemplate()

	fmt.Fprint(os.Stderr, "Generating PR description...")
	body, err := ai.FillPRBody(template, string(diff), string(commits))
	if err != nil {
		fmt.Fprintf(os.Stderr, " failed (%v)\n", err)
		return
	}
	fmt.Fprintln(os.Stderr, " done.")

	applyPRBody(body)
}

func applyPRBody(body string) {
	cmd := exec.Command("gh", "pr", "edit", "--body-file", "-")
	cmd.Stdin = strings.NewReader(body)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "Could not update PR body via gh, copying to clipboard instead.")
		if !copyToClipboard(body) {
			fmt.Fprintln(os.Stderr, "---")
			fmt.Fprintln(os.Stderr, body)
			fmt.Fprintln(os.Stderr, "---")
		}
		return
	}
	fmt.Fprintln(os.Stderr, "PR description updated.")
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
