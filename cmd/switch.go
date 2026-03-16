package cmd

import (
	"github.com/santhosh/mgt/pkg/gt"
	"github.com/spf13/cobra"
)

var switchCmd = &cobra.Command{
	Use:                "switch [branch]",
	Short:              "Interactively switch to any branch",
	Long:               "Shows an interactive branch picker to quickly jump to any branch. If a branch name is given, switches directly to it.",
	Aliases:            []string{"sw"},
	DisableFlagParsing: true,
	Run: func(cmd *cobra.Command, args []string) {
		gt.Exec(append([]string{"branch", "checkout"}, args...)...)
	},
}

func init() {
	rootCmd.AddCommand(switchCmd)
}
