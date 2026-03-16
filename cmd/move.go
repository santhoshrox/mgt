package cmd

import (
	"github.com/santhosh/mgt/pkg/gt"
	"github.com/spf13/cobra"
)

var moveCmd = &cobra.Command{
	Use:                "move [parent-branch]",
	Short:              "Move current branch onto a new parent",
	Long:               "Rebases the current branch and all its descendants onto a new parent. Wraps `gt upstack onto`. If no parent is given, charcoal shows an interactive branch picker.",
	DisableFlagParsing: true,
	Run: func(cmd *cobra.Command, args []string) {
		gt.Exec(append([]string{"upstack", "onto"}, args...)...)
	},
}

func init() {
	rootCmd.AddCommand(moveCmd)
}
