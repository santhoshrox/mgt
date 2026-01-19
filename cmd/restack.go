package cmd

import (
	"fmt"

	"github.com/santhosh/mgt/pkg/gt"
	"github.com/spf13/cobra"
)

var restackCmd = &cobra.Command{
	Use:   "restack",
	Short: "Sync with latest trunk and restack current branch + ancestors",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Syncing with remote trunk...")
		if err := gt.Run("repo", "sync"); err != nil {
			fmt.Printf("Error during sync: %v\n", err)
			return
		}

		fmt.Println("Restacking current branch and ancestors...")
		if err := gt.Run("downstack", "restack"); err != nil {
			fmt.Printf("Error during restack: %v\n", err)
			return
		}
	},
}

func init() {
	rootCmd.AddCommand(restackCmd)
}
