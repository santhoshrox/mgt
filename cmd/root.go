package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "mgt",
	Short: "mgt is an enhanced wrapper for the gt (Charcoal) CLI",
	Long:  `mgt enhances the Charcoal (gt) tool with simpler navigation and better stack management from the trunk.`,
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Add flags if needed
}
