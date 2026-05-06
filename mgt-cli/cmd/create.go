package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/santhoshrox/mgt/pkg/client"
	"github.com/santhoshrox/mgt/pkg/config"
	"github.com/santhoshrox/mgt/pkg/git"
	"github.com/santhoshrox/mgt/pkg/repo"
	"github.com/spf13/cobra"
)

var createCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "Create a new branch stacked on top of the current branch",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		raw := strings.TrimSpace(args[0])
		name := config.BranchPrefix() + raw

		c := client.New()
		r, err := repo.Resolve(c)
		if err != nil {
			exit(err)
		}
		current, err := git.CurrentBranch()
		if err != nil {
			exit(err)
		}

		// Find or create the stack the new branch belongs to.
		st, _ := c.StackByBranch(r.ID, current)
		if st.ID == 0 {
			// Current branch isn't in any stack yet. If we're on trunk, the
			// new branch becomes a fresh stack rooted on trunk; otherwise
			// it goes into a new stack with `current` as its first branch.
			seed := current
			if current == st.TrunkBranch || current == r.DefaultBranch {
				seed = ""
			}
			st, err = c.CreateStack(r.ID, seed)
			if err != nil {
				exit(err)
			}
		}

		if err := git.CreateBranch(name); err != nil {
			exit(err)
		}

		parent := current
		if current == st.TrunkBranch || current == r.DefaultBranch {
			parent = ""
		}
		if _, err := c.AddBranch(r.ID, st.ID, client.AddBranch{Name: name, Parent: parent}); err != nil {
			fmt.Fprintf(os.Stderr, "mgt: created branch %s locally but server registration failed: %v\n", name, err)
			os.Exit(1)
		}
		fmt.Printf("Created %s on top of %s.\n", name, current)
	},
}

func init() {
	rootCmd.AddCommand(createCmd)
}
