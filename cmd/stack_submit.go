package cmd

import (
	"fmt"
	"os"

	"github.com/santhosh/mgt/pkg/gt"
	"github.com/spf13/cobra"
)

var stackSubmitCmd = &cobra.Command{
	Use:                "stack-submit",
	Short:              "Submit the entire current stack as pull requests",
	DisableFlagParsing: true,
	Args:               cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		wantAI := promptForAI()

		gtArgs := []string{"stack", "submit"}
		gtArgs = append(gtArgs, args...)

		if wantAI {
			if err := gt.Run(gtArgs...); err != nil {
				os.Exit(1)
			}
			describeStack()
		} else {
			gt.Exec(gtArgs...)
		}
	},
}

func describeStack() {
	trunk, err := gt.GetTrunk()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error detecting trunk: %v\n", err)
		return
	}
	branches, err := gt.GetStackBranches()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting stack branches: %v\n", err)
		return
	}

	template := findPRTemplate()

	for i, branch := range branches {
		parent := trunk
		if i > 0 {
			parent = branches[i-1]
		}
		fmt.Fprintf(os.Stderr, "Generating PR description for %s...", branch)
		body, err := generateDescription(branch, parent, template)
		if err != nil {
			fmt.Fprintf(os.Stderr, " failed: %v\n", err)
			continue
		}
		if body == "" {
			fmt.Fprintln(os.Stderr, " no changes, skipped.")
			continue
		}
		fmt.Fprintln(os.Stderr, " done.")
		applyPRBody(branch, body)
	}
}

func init() {
	rootCmd.AddCommand(stackSubmitCmd)
}
