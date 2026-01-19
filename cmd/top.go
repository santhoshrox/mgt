package cmd

import (
	"github.com/santhosh/mgt/pkg/gt"
	"github.com/spf13/cobra"
)

var topCmd = &cobra.Command{
	Use:   "top",
	Short: "Go to the top of the current stack",
	Run: func(cmd *cobra.Command, args []string) {
		gt.Run("branch", "top")
	},
}

func init() {
	rootCmd.AddCommand(topCmd)
}
