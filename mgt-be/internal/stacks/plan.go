// Package stacks holds pure logic for transforming a stack tree (move,
// extract, reorder) and computing the rebase plan the CLI must run.
package stacks

import (
	"fmt"

	"github.com/santhoshrox/mgt-be/internal/db"
)

// RebaseStep is the wire shape returned to the CLI after a stack mutation.
type RebaseStep struct {
	Branch string `json:"branch"`
	Onto   string `json:"onto"`
}

// MovePlan is what the API returns to the CLI plus the DB mutations the
// caller should persist.
type MovePlan struct {
	DBUpdates   []DBUpdate
	RebaseSteps []RebaseStep
}

type DBUpdate struct {
	BranchID int64
	ParentID *int64
	Position int
}

// PlanMove repoints `targetID` to live on top of `newParentName` (or trunk if
// empty). Children of the target follow it. Children of its previous parent
// are reconnected to that parent (i.e. extract).
func PlanMove(branches []db.StackBranch, targetID int64, newParentName string) (MovePlan, error) {
	byID := map[int64]db.StackBranch{}
	byName := map[string]db.StackBranch{}
	for _, b := range branches {
		byID[b.ID] = b
		byName[b.Name] = b
	}
	target, ok := byID[targetID]
	if !ok {
		return MovePlan{}, fmt.Errorf("branch %d not found in stack", targetID)
	}

	var newParentID *int64
	newParentBranchName := "" // empty means trunk
	if newParentName != "" {
		p, ok := byName[newParentName]
		if !ok {
			return MovePlan{}, fmt.Errorf("parent branch %s not in stack", newParentName)
		}
		id := p.ID
		newParentID = &id
		newParentBranchName = p.Name
		cur := &p
		for cur != nil {
			if cur.ID == targetID {
				return MovePlan{}, fmt.Errorf("cannot move %s onto its descendant %s", target.Name, newParentName)
			}
			if cur.ParentID == nil {
				break
			}
			next := byID[*cur.ParentID]
			cur = &next
		}
	}

	plan := MovePlan{}

	// Reparent target's direct children onto target's previous parent (extract).
	prevParentID := target.ParentID
	prevParentName := ""
	if prevParentID != nil {
		prevParentName = byID[*prevParentID].Name
	}
	for _, b := range branches {
		if b.ParentID != nil && *b.ParentID == targetID && b.ID != targetID {
			plan.DBUpdates = append(plan.DBUpdates, DBUpdate{BranchID: b.ID, ParentID: prevParentID, Position: b.Position})
			plan.RebaseSteps = append(plan.RebaseSteps, RebaseStep{Branch: b.Name, Onto: prevParentName})
		}
	}

	// Move the target itself.
	plan.DBUpdates = append(plan.DBUpdates, DBUpdate{BranchID: target.ID, ParentID: newParentID, Position: target.Position})
	plan.RebaseSteps = append(plan.RebaseSteps, RebaseStep{Branch: target.Name, Onto: newParentBranchName})

	return plan, nil
}
