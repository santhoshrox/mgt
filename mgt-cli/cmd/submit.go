package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/santhosh/mgt/pkg/client"
	"github.com/santhosh/mgt/pkg/git"
	"github.com/santhosh/mgt/pkg/repo"
	"github.com/spf13/cobra"
)

var submitAI bool

var submitCmd = &cobra.Command{
	Use:   "submit",
	Short: "Push the current branch and create/update its PR on mgt-be",
	Run: func(cmd *cobra.Command, args []string) {
		c := client.New()
		r, err := repo.Resolve(c)
		if err != nil {
			exit(err)
		}
		current, err := git.CurrentBranch()
		if err != nil {
			exit(err)
		}
		if current == r.DefaultBranch {
			exit(fmt.Errorf("cannot submit trunk (%s)", r.DefaultBranch))
		}
		st, _ := c.StackByBranch(r.ID, current)
		base := r.DefaultBranch
		for _, b := range st.Branches {
			if b.Name == current {
				if b.Parent != "" {
					base = b.Parent
				}
			}
		}

		fmt.Printf("Pushing %s to origin...\n", current)
		if err := git.Push("origin", current); err != nil {
			exit(err)
		}

		title := titleFromBranch(current)
		fmt.Printf("Creating/updating PR for %s onto %s...\n", current, base)
		pr, err := c.CreatePR(r.ID, client.CreatePR{
			Branch: current, Base: base, Title: title,
		}, submitAI)
		if err != nil {
			exit(err)
		}
		fmt.Printf("PR #%d %s\n", pr.Number, pr.HTMLURL)
	},
}

var stackSubmitCmd = &cobra.Command{
	Use:   "stack-submit",
	Short: "Push every branch in the current stack and create/update PRs",
	Run: func(cmd *cobra.Command, args []string) {
		c := client.New()
		r, err := repo.Resolve(c)
		if err != nil {
			exit(err)
		}
		current, err := git.CurrentBranch()
		if err != nil {
			exit(err)
		}
		st, _ := c.StackByBranch(r.ID, current)
		if len(st.Branches) == 0 {
			exit(fmt.Errorf("no branches in current stack"))
		}

		for _, b := range st.Branches {
			base := b.Parent
			if base == "" {
				base = r.DefaultBranch
			}
			fmt.Printf("Pushing %s...\n", b.Name)
			if err := git.Push("origin", b.Name); err != nil {
				fmt.Fprintf(os.Stderr, "mgt: push %s failed: %v\n", b.Name, err)
				continue
			}
			pr, err := c.CreatePR(r.ID, client.CreatePR{
				Branch: b.Name, Base: base, Title: titleFromBranch(b.Name),
			}, submitAI)
			if err != nil {
				fmt.Fprintf(os.Stderr, "mgt: PR for %s failed: %v\n", b.Name, err)
				continue
			}
			fmt.Printf("  → PR #%d %s\n", pr.Number, pr.HTMLURL)
		}
	},
}

// titleFromBranch turns "santhosh/add-cool-feature" into "Add cool feature".
func titleFromBranch(branch string) string {
	parts := strings.Split(branch, "/")
	last := parts[len(parts)-1]
	last = strings.ReplaceAll(last, "-", " ")
	last = strings.ReplaceAll(last, "_", " ")
	if last == "" {
		return branch
	}
	return strings.ToUpper(last[:1]) + last[1:]
}

func init() {
	submitCmd.Flags().BoolVar(&submitAI, "ai", false, "fill the PR body via the server-configured LLM")
	stackSubmitCmd.Flags().BoolVar(&submitAI, "ai", false, "fill PR bodies via the server-configured LLM")
	rootCmd.AddCommand(submitCmd, stackSubmitCmd)
}
