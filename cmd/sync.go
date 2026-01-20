package cmd

import (
	"github.com/santhosh/mgt/pkg/gt"
	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
	Use:     "sync",
	Aliases: []string{"cleanup", "prune"},
	Short:   "Sync trunk and cleanup merged branches",
	Run: func(cmd *cobra.Command, args []string) {
		gt.Exec("repo", "sync")
	},
}

func init() {
	rootCmd.AddCommand(syncCmd)
}
