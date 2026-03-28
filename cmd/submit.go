package cmd

import (
	"fmt"
	"os"

	"github.com/santhosh/mgt/pkg/gt"
	"github.com/spf13/cobra"
)

var submitCmd = &cobra.Command{
	Use:                "submit",
	Short:              "Submit the current branch as a pull request",
	DisableFlagParsing: true,
	Args:               cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		wantAI := promptForAI()

		gtArgs := []string{"branch", "submit"}
		gtArgs = append(gtArgs, args...)

		if wantAI {
			if err := gt.Run(gtArgs...); err != nil {
				os.Exit(1)
			}
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
				return
			}
			fmt.Fprintln(os.Stderr, " done.")
			applyPRBody(current, body)
		} else {
			gt.Exec(gtArgs...)
		}
	},
}

func init() {
	rootCmd.AddCommand(submitCmd)
}
