package cmd

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/santhosh/mgt/pkg/client"
	"github.com/santhosh/mgt/pkg/git"
	"github.com/santhosh/mgt/pkg/repo"
	"github.com/spf13/cobra"
)

// applyPlan executes a server-issued RebasePlan by running `git rebase --onto`
// for each step. trunk is substituted in for empty Onto values.
func applyPlan(plan client.RebasePlan, trunk string) error {
	for _, step := range plan.Steps {
		onto := step.Onto
		if onto == "" {
			onto = trunk
		}
		fmt.Printf("Rebasing %s onto %s...\n", step.Branch, onto)
		if err := git.CheckoutQuiet(step.Branch); err != nil {
			return err
		}
		if err := git.Run("rebase", onto); err != nil {
			return err
		}
	}
	return nil
}

// findBranchID returns the stack-branch id for `name` in the supplied stack.
func findBranchID(st client.Stack, name string) (int64, bool) {
	for _, b := range st.Branches {
		if b.Name == name {
			return b.ID, true
		}
	}
	return 0, false
}

// ── move ─────────────────────────────────────────────────────────────────

var moveCmd = &cobra.Command{
	Use:   "move [parent-branch]",
	Short: "Move the current branch onto a new parent",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		newParent := strings.TrimSpace(args[0])
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
		if err != nil {
			exit(err)
		}
		bid, ok := findBranchID(st, cur)
		if !ok {
			exit(fmt.Errorf("%s is not in a tracked stack", cur))
		}
		plan, err := c.MoveBranch(r.ID, st.ID, bid, client.MoveBranch{Parent: newParent})
		if err != nil {
			exit(err)
		}
		if err := applyPlan(plan, st.TrunkBranch); err != nil {
			exit(err)
		}
		_ = git.CheckoutQuiet(cur)
	},
}

// ── extract ───────────────────────────────────────────────────────────────

var extractCmd = &cobra.Command{
	Use:   "extract",
	Short: "Detach the current branch from its stack onto trunk as a standalone stack",
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
		if err != nil {
			exit(err)
		}
		bid, ok := findBranchID(st, cur)
		if !ok {
			exit(fmt.Errorf("%s is not in a tracked stack", cur))
		}
		// Empty Parent means "onto trunk".
		plan, err := c.MoveBranch(r.ID, st.ID, bid, client.MoveBranch{Parent: ""})
		if err != nil {
			exit(err)
		}
		if err := applyPlan(plan, st.TrunkBranch); err != nil {
			exit(err)
		}
		_ = git.CheckoutQuiet(cur)
	},
}

// ── reorder ──────────────────────────────────────────────────────────────

type reorderModel struct {
	branches  []string // tip-first, last entry sits on trunk
	cursor    int
	confirmed bool
}

func (m reorderModel) Init() tea.Cmd { return nil }

func (m reorderModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "ctrl+c", "q", "esc":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.branches)-1 {
				m.cursor++
			}
		case "K", "shift+up":
			if m.cursor > 0 {
				m.branches[m.cursor], m.branches[m.cursor-1] = m.branches[m.cursor-1], m.branches[m.cursor]
				m.cursor--
			}
		case "J", "shift+down":
			if m.cursor < len(m.branches)-1 {
				m.branches[m.cursor], m.branches[m.cursor+1] = m.branches[m.cursor+1], m.branches[m.cursor]
				m.cursor++
			}
		case "enter":
			m.confirmed = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m reorderModel) View() string {
	var sb strings.Builder
	sb.WriteString("Reorder stack  ↑/↓ move cursor  K/J move branch  Enter confirm  q cancel\n\n")
	for i, b := range m.branches {
		marker := "  "
		if i == m.cursor {
			marker = "▶ "
		}
		sb.WriteString(marker + "◉ " + b + "\n")
	}
	sb.WriteString("  ◯ (trunk)\n")
	return sb.String()
}

var reorderCmd = &cobra.Command{
	Use:   "reorder",
	Short: "Interactively reorder the branches in the current stack",
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
		if err != nil {
			exit(err)
		}
		if len(st.Branches) < 2 {
			fmt.Println("Need at least 2 branches in the stack to reorder.")
			return
		}
		// Branches arrive base→tip; flip to tip→base for the picker.
		display := make([]string, len(st.Branches))
		for i, b := range st.Branches {
			display[len(st.Branches)-1-i] = b.Name
		}
		result, err := tea.NewProgram(reorderModel{branches: display}).Run()
		if err != nil {
			exit(err)
		}
		final := result.(reorderModel)
		if !final.confirmed {
			fmt.Println("Cancelled.")
			return
		}
		// Execute moves base→tip so that each branch is rebased onto its new parent.
		newOrder := make([]string, len(final.branches))
		for i, b := range final.branches {
			newOrder[len(final.branches)-1-i] = b
		}
		for i, branchName := range newOrder {
			parent := ""
			if i > 0 {
				parent = newOrder[i-1]
			}
			st2, err := c.StackByBranch(r.ID, branchName)
			if err != nil {
				exit(err)
			}
			bid, ok := findBranchID(st2, branchName)
			if !ok {
				continue
			}
			plan, err := c.MoveBranch(r.ID, st2.ID, bid, client.MoveBranch{Parent: parent})
			if err != nil {
				fmt.Fprintf(os.Stderr, "mgt: move %s failed: %v\n", branchName, err)
				continue
			}
			if err := applyPlan(plan, st.TrunkBranch); err != nil {
				exit(err)
			}
		}
		_ = git.CheckoutQuiet(cur)
		fmt.Printf("Done. Back on %s.\n", cur)
	},
}

func init() {
	rootCmd.AddCommand(moveCmd, extractCmd, reorderCmd)
}
