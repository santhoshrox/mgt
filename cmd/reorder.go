package cmd

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/santhosh/mgt/pkg/gt"
	"github.com/spf13/cobra"
)

type reorderModel struct {
	// branches stored in display order: index 0 = tip (top of screen), last = base (closest to trunk)
	branches  []string
	cursor    int
	confirmed bool
}

func newReorderModel(baseToBranches []string) reorderModel {
	// Reverse base→tip to tip→base for display
	display := make([]string, len(baseToBranches))
	for i, b := range baseToBranches {
		display[len(baseToBranches)-1-i] = b
	}
	return reorderModel{branches: display}
}

func (m reorderModel) Init() tea.Cmd { return nil }

func (m reorderModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
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
			// Move selected branch up on screen (toward tip)
			if m.cursor > 0 {
				m.branches[m.cursor], m.branches[m.cursor-1] = m.branches[m.cursor-1], m.branches[m.cursor]
				m.cursor--
			}
		case "J", "shift+down":
			// Move selected branch down on screen (toward trunk)
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
		if i == m.cursor {
			sb.WriteString(fmt.Sprintf("▶ ◉ %s\n", b))
		} else {
			sb.WriteString(fmt.Sprintf("  ◉ %s\n", b))
		}
	}
	sb.WriteString("  ◯ (trunk)\n")
	return sb.String()
}

var reorderCmd = &cobra.Command{
	Use:   "reorder",
	Short: "Interactively reorder branches in the current stack",
	Long:  "Shows a TUI to drag-reorder multiple branches in the stack, then executes the necessary `gt upstack onto` moves.",
	Run: func(cmd *cobra.Command, args []string) {
		current, err := gt.GetCurrentBranch()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting current branch: %v\n", err)
			os.Exit(1)
		}

		trunk, err := gt.GetTrunk()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error detecting trunk: %v\n", err)
			os.Exit(1)
		}

		branches, err := gt.GetStackBranches()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting stack branches: %v\n", err)
			os.Exit(1)
		}

		if len(branches) < 2 {
			fmt.Println("Need at least 2 branches in the stack to reorder. Use `mgt move` to reparent a single branch.")
			return
		}

		m := newReorderModel(branches)
		p := tea.NewProgram(m)
		result, err := p.Run()
		if err != nil {
			fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
			os.Exit(1)
		}

		final := result.(reorderModel)
		if !final.confirmed {
			fmt.Println("Cancelled.")
			return
		}

		// Convert display order (tip→base) back to execution order (base→tip)
		display := final.branches
		newOrder := make([]string, len(display))
		for i, b := range display {
			newOrder[len(display)-1-i] = b
		}

		fmt.Println("Reordering stack...")
		if err := gt.ExecuteReorder(trunk, newOrder, current); err != nil {
			fmt.Fprintf(os.Stderr, "Reorder failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Done. Back on %s.\n", current)
	},
}

func init() {
	rootCmd.AddCommand(reorderCmd)
}
