package cmd

import (
	"fmt"
	"os"

	"github.com/santhosh/mgt/pkg/config"
	"github.com/santhosh/mgt/pkg/gt"
	"github.com/spf13/cobra"
)

var createCmd = &cobra.Command{
	Use:                "create [name]",
	Short:              "Create a new branch stacked on top of the current branch",
	Args:               cobra.ArbitraryArgs,
	DisableFlagParsing: true,
	Run: func(cmd *cobra.Command, args []string) {
		prefix := config.BranchPrefix()
		gtArgs := []string{"branch", "create"}

		if prefix != "" && len(args) > 0 {
			// Pass name without prefix so gt can add its own prefix (e.g. date) first.
			// After gt returns we rename to prefix+<actual-branch-name> so result is e.g. santhosh/02-16-feature.
			gtArgs = append(gtArgs, args...)
			if err := gt.Run(gtArgs...); err != nil {
				os.Exit(1)
			}
			current, err := gt.GetCurrentBranch()
			if err != nil {
				fmt.Fprintf(os.Stderr, "mgt: could not get current branch: %v\n", err)
				os.Exit(1)
			}
			newName := prefix + current
			if err := gt.RenameCurrentBranch(newName); err != nil {
				fmt.Fprintf(os.Stderr, "mgt: could not rename branch to %q: %v\n", newName, err)
				os.Exit(1)
			}
			return
		}

		gtArgs = append(gtArgs, args...)
		gt.Exec(gtArgs...)
	},
}

func init() {
	rootCmd.AddCommand(createCmd)
}
