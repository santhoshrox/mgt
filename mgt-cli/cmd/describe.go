package cmd

import (
	"fmt"

	"github.com/santhoshrox/mgt/pkg/client"
	"github.com/santhoshrox/mgt/pkg/git"
	"github.com/santhoshrox/mgt/pkg/repo"
	"github.com/spf13/cobra"
)

var describeCmd = &cobra.Command{
	Use:   "describe",
	Short: "Have the server fill in the current PR's description via LLM",
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
		// Find the open PR for this branch (search by head_branch).
		prs, err := c.ListPRs(r.ID, "open", "", "")
		if err != nil {
			exit(err)
		}
		var num int
		for _, p := range prs {
			if p.HeadBranch == cur {
				num = p.Number
				break
			}
		}
		if num == 0 {
			exit(fmt.Errorf("no open PR for %s — run `mgt submit` first", cur))
		}

		body, err := c.DescribePR(r.ID, num)
		if err != nil {
			exit(err)
		}
		fmt.Printf("Updated PR #%d description.\n\n%s\n", num, body)
	},
}

func init() {
	rootCmd.AddCommand(describeCmd)
}
