package cmd

import (
	"fmt"

	"github.com/santhosh/mgt/pkg/client"
	"github.com/santhosh/mgt/pkg/git"
	"github.com/santhosh/mgt/pkg/repo"
	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
	Use:     "sync",
	Aliases: []string{"cleanup", "prune"},
	Short:   "Pull trunk and delete local branches whose PRs are merged",
	Run: func(cmd *cobra.Command, args []string) {
		c := client.New()
		r, err := repo.Resolve(c)
		if err != nil {
			exit(err)
		}
		fmt.Println("Fetching from origin...")
		if err := git.FetchAll(); err != nil {
			exit(err)
		}
		if err := git.CheckoutQuiet(r.DefaultBranch); err != nil {
			exit(err)
		}
		if err := git.Run("pull", "origin", r.DefaultBranch); err != nil {
			exit(err)
		}

		// Ask the server which PRs are merged so we can prune locally.
		prs, err := c.ListPRs(r.ID, "closed", "", "")
		if err != nil {
			exit(err)
		}
		mergedBranches := map[string]int{}
		for _, p := range prs {
			if p.Merged {
				mergedBranches[p.HeadBranch] = p.Number
			}
		}
		if len(mergedBranches) == 0 {
			fmt.Println("No merged branches to clean up.")
			return
		}
		for branch, num := range mergedBranches {
			fmt.Printf("Deleting local branch %s (#%d merged)...\n", branch, num)
			_ = git.DeleteBranch(branch)
		}
	},
}

var restackCmd = &cobra.Command{
	Use:   "restack",
	Short: "Pull trunk and rebase the current stack onto it",
	Run: func(cmd *cobra.Command, args []string) {
		c := client.New()
		r, err := repo.Resolve(c)
		if err != nil {
			exit(err)
		}
		cur, err := git.CurrentBranch()
		if err != nil {
			exit(err)
		}
		st, err := c.StackByBranch(r.ID, cur)
		if err != nil || len(st.Branches) == 0 {
			exit(fmt.Errorf("current branch is not in a stack"))
		}
		if err := git.FetchAll(); err != nil {
			exit(err)
		}
		// Rebase each branch (in stored order, base→tip) onto its parent.
		for _, b := range st.Branches {
			parent := b.Parent
			if parent == "" {
				parent = "origin/" + st.TrunkBranch
			}
			fmt.Printf("Rebasing %s onto %s...\n", b.Name, parent)
			if err := git.CheckoutQuiet(b.Name); err != nil {
				exit(err)
			}
			if err := git.Run("rebase", parent); err != nil {
				exit(err)
			}
		}
		_ = git.CheckoutQuiet(cur)
	},
}

func init() {
	rootCmd.AddCommand(syncCmd, restackCmd)
}
