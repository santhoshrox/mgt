package cmd

import (
	"fmt"
	"os"

	"github.com/santhosh/mgt/pkg/gt"
	"github.com/spf13/cobra"
)

var extractCmd = &cobra.Command{
	Use:   "extract",
	Short: "Extract current branch from its stack onto trunk as a separate stack",
	Long:  "Detaches the current branch from its stack, reconnects any children to the branch's parent, and moves the branch directly onto trunk as a standalone stack.",
	Run: func(cmd *cobra.Command, args []string) {
		current, err := gt.GetCurrentBranch()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting current branch: %v\n", err)
			os.Exit(1)
		}

		trunk, err := gt.GetTrunk()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error detecting trunk: %v\n", err)
			os.Exit(1)
		}

		if current == trunk {
			fmt.Fprintln(os.Stderr, "Already on trunk. Switch to the branch you want to extract.")
			os.Exit(1)
		}

		branches, err := gt.GetStackBranches()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting stack branches: %v\n", err)
			os.Exit(1)
		}

		if len(branches) < 2 {
			fmt.Println("Branch is already a standalone stack on trunk.")
			return
		}

		fmt.Printf("Extracting %s onto %s as a separate stack...\n", current, trunk)
		if err := gt.ExtractBranch(trunk, branches, current); err != nil {
			fmt.Fprintf(os.Stderr, "Extract failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Done. %s is now a standalone stack on %s.\n", current, trunk)
	},
}

func init() {
	rootCmd.AddCommand(extractCmd)
}
