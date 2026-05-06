package cmd

import (
	"fmt"

	"github.com/santhosh/mgt/pkg/client"
	"github.com/spf13/cobra"
)

var syncReposCmd = &cobra.Command{
	Use:   "sync-repos",
	Short: "Refresh the list of repositories on mgt-be from GitHub",
	Run: func(cmd *cobra.Command, args []string) {
		c := client.New()
		repos, err := c.SyncRepos()
		if err != nil {
			exit(err)
		}
		fmt.Printf("%d repositories registered.\n", len(repos))
		for _, r := range repos {
			fmt.Printf("  %s (default: %s)\n", r.FullName, r.DefaultBranch)
		}
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the current branch's stack as known to mgt-be",
	Run: func(cmd *cobra.Command, args []string) {
		sc, err := loadStackContext()
		if err != nil {
			exit(err)
		}
		fmt.Printf("Repo:   %s\n", sc.repo.FullName)
		fmt.Printf("Trunk:  %s\n", sc.stack.TrunkBranch)
		fmt.Printf("Branch: %s\n", sc.current)
		if len(sc.stack.Branches) == 0 {
			fmt.Println("(no stack)")
			return
		}
		fmt.Println("Stack:")
		for _, b := range sc.stack.Branches {
			marker := "  "
			if b.Name == sc.current {
				marker = "▶ "
			}
			fmt.Printf("%s%s  (parent: %s)\n", marker, b.Name, ifEmpty(b.Parent, sc.stack.TrunkBranch))
		}
	},
}

func ifEmpty(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func init() {
	rootCmd.AddCommand(syncReposCmd, statusCmd)
}
