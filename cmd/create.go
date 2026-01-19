package cmd

import (
	"github.com/santhosh/mgt/pkg/gt"
	"github.com/spf13/cobra"
)

var createCmd = &cobra.Command{
	Use:                "create [name]",
	Short:              "Create a new branch stacked on top of the current branch",
	Args:               cobra.ArbitraryArgs,
	DisableFlagParsing: true,
	Run: func(cmd *cobra.Command, args []string) {
		gtArgs := []string{"branch", "create"}
		gtArgs = append(gtArgs, args...)
		gt.Exec(gtArgs...)
	},
}

func init() {
	rootCmd.AddCommand(createCmd)
}
