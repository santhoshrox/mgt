package cmd

import (
	"github.com/santhosh/mgt/pkg/gt"
	"github.com/spf13/cobra"
)

var downCmd = &cobra.Command{
	Use:   "down",
	Short: "Go down the stack (towards trunk)",
	Run: func(cmd *cobra.Command, args []string) {
		gt.Run("branch", "down")
	},
}

func init() {
	rootCmd.AddCommand(downCmd)
}
