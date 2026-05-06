package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "mgt",
	Short: "mgt is a stacked-PR CLI that talks to a self-hosted mgt-be server",
	Long:  `mgt manages stacked pull requests against a self-hosted mgt-be backend. Stack metadata, PRs, AI descriptions, and the merge queue all live on the server; the CLI just runs git locally.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
