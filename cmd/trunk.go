package cmd

import (
	"fmt"

	"github.com/santhosh/mgt/pkg/gt"
	"github.com/spf13/cobra"
)

var trunkCmd = &cobra.Command{
	Use:   "trunk",
	Short: "Switch to the trunk branch",
	Run: func(cmd *cobra.Command, args []string) {
		trunk, err := gt.GetTrunk()
		if err != nil {
			fmt.Printf("Error detecting trunk: %v\n", err)
			return
		}
		fmt.Printf("Switching to trunk (%s)...\n", trunk)
		gt.Run("branch", "checkout", trunk)
	},
}

func init() {
	rootCmd.AddCommand(trunkCmd)
}
