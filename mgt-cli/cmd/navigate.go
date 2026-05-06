package cmd

import (
	"fmt"
	"os"

	"github.com/santhosh/mgt/pkg/client"
	"github.com/santhosh/mgt/pkg/git"
	"github.com/santhosh/mgt/pkg/repo"
	"github.com/spf13/cobra"
)

// stackContext holds everything a navigation command needs.
type stackContext struct {
	client  *client.Client
	repo    client.Repo
	current string
	stack   client.Stack
}

func loadStackContext() (stackContext, error) {
	var sc stackContext
	if err := git.EnsureRepo(); err != nil {
		return sc, err
	}
	c := client.New()
	r, err := repo.Resolve(c)
	if err != nil {
		return sc, err
	}
	cur, err := git.CurrentBranch()
	if err != nil {
		return sc, err
	}
	st, _ := c.StackByBranch(r.ID, cur)
	sc.client, sc.repo, sc.current, sc.stack = c, r, cur, st
	return sc, nil
}

// childrenOf returns the names of branches whose parent is `name`. If name is
// the trunk, returns the roots of the stack.
func (sc stackContext) childrenOf(name string) []string {
	var out []string
	for _, b := range sc.stack.Branches {
		if (name == sc.stack.TrunkBranch && b.Parent == sc.stack.TrunkBranch) || b.Parent == name {
			out = append(out, b.Name)
		}
	}
	return out
}

// parentOf returns the parent name of branch `name` (or trunk).
func (sc stackContext) parentOf(name string) string {
	for _, b := range sc.stack.Branches {
		if b.Name == name {
			return b.Parent
		}
	}
	return sc.stack.TrunkBranch
}

// pickInteractive shows a numbered list and returns the chosen branch (1-based).
func pickInteractive(prompt string, options []string) (string, error) {
	fmt.Fprintln(os.Stderr, prompt)
	for i, b := range options {
		fmt.Fprintf(os.Stderr, "  %d) %s\n", i+1, b)
	}
	fmt.Fprint(os.Stderr, "Choose [1]: ")
	var pick int
	if _, err := fmt.Fscanln(os.Stdin, &pick); err != nil {
		pick = 1
	}
	if pick < 1 || pick > len(options) {
		return "", fmt.Errorf("invalid selection")
	}
	return options[pick-1], nil
}

// ── Commands ──────────────────────────────────────────────────────────────

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Move toward the tip of the stack",
	Run: func(cmd *cobra.Command, args []string) {
		sc, err := loadStackContext()
		if err != nil {
			exit(err)
		}
		kids := sc.childrenOf(sc.current)
		if len(kids) == 0 {
			fmt.Println("Already at the tip of the stack.")
			return
		}
		next := kids[0]
		if len(kids) > 1 {
			next, err = pickInteractive("Multiple children — pick one:", kids)
			if err != nil {
				exit(err)
			}
		}
		exit(git.CheckoutQuiet(next))
	},
}

var downCmd = &cobra.Command{
	Use:   "down",
	Short: "Move toward trunk",
	Run: func(cmd *cobra.Command, args []string) {
		sc, err := loadStackContext()
		if err != nil {
			exit(err)
		}
		parent := sc.parentOf(sc.current)
		if parent == "" || parent == sc.current {
			parent = sc.stack.TrunkBranch
		}
		exit(git.CheckoutQuiet(parent))
	},
}

var topCmd = &cobra.Command{
	Use:   "top",
	Short: "Jump to the tip of the current stack",
	Run: func(cmd *cobra.Command, args []string) {
		sc, err := loadStackContext()
		if err != nil {
			exit(err)
		}
		// Walk up until no children.
		cur := sc.current
		visited := map[string]bool{cur: true}
		for {
			kids := sc.childrenOf(cur)
			if len(kids) == 0 {
				break
			}
			cur = kids[0]
			if visited[cur] {
				break
			}
			visited[cur] = true
		}
		if cur == sc.current {
			fmt.Println("Already at the tip.")
			return
		}
		exit(git.CheckoutQuiet(cur))
	},
}

var trunkCmd = &cobra.Command{
	Use:   "trunk",
	Short: "Switch to the trunk branch",
	Run: func(cmd *cobra.Command, args []string) {
		sc, err := loadStackContext()
		if err != nil {
			exit(err)
		}
		fmt.Printf("Switching to %s...\n", sc.stack.TrunkBranch)
		exit(git.CheckoutQuiet(sc.stack.TrunkBranch))
	},
}

var switchCmd = &cobra.Command{
	Use:     "switch [branch]",
	Aliases: []string{"sw"},
	Short:   "Switch to any branch (interactive picker if no name given)",
	Args:    cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 1 {
			exit(git.CheckoutQuiet(args[0]))
		}
		sc, err := loadStackContext()
		if err != nil {
			exit(err)
		}
		options := []string{sc.stack.TrunkBranch}
		for _, b := range sc.stack.Branches {
			options = append(options, b.Name)
		}
		pick, err := pickInteractive("Pick a branch:", options)
		if err != nil {
			exit(err)
		}
		exit(git.CheckoutQuiet(pick))
	},
}

func exit(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "mgt: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(upCmd, downCmd, topCmd, trunkCmd, switchCmd)
}
