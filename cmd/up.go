package cmd

import (
	"fmt"

	"github.com/santhosh/mgt/pkg/gt"
	"github.com/spf13/cobra"
)

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Go up the stack or select a stack from trunk",
	Run: func(cmd *cobra.Command, args []string) {
		current, err := gt.GetCurrentBranch()
		if err != nil {
			fmt.Printf("Error getting current branch: %v\n", err)
			return
		}

		trunk, err := gt.GetTrunk()
		if err != nil {
			fmt.Printf("Error detecting trunk: %v\n", err)
			return
		}

		if current == trunk {
			// gt branch checkout displays an interactive selector when no branch is provided
			gt.Run("branch", "checkout")
		} else {
			gt.Run("branch", "up")
		}
	},
}

func init() {
	rootCmd.AddCommand(upCmd)
}
