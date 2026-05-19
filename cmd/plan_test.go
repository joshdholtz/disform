package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/joshholtz/disform/internal/planner"
)

func planOutput(plan *planner.Plan) string {
	var buf bytes.Buffer
	printPlanTo(&buf, plan)
	return buf.String()
}

func TestPrintPlanCreateAction(t *testing.T) {
	plan := &planner.Plan{
		ToCreate: 1,
		Actions: []planner.Action{
			{Type: planner.ActionCreate, ResourceType: planner.ResourceRole, Name: "admin"},
		},
	}
	out := planOutput(plan)
	if !strings.Contains(out, `"admin" will be created`) {
		t.Errorf("expected create line, got:\n%s", out)
	}
}

func TestPrintPlanUpdateAction(t *testing.T) {
	plan := &planner.Plan{
		ToUpdate: 1,
		Actions: []planner.Action{
			{
				Type:         planner.ActionUpdate,
				ResourceType: planner.ResourceChannel,
				Name:         "General/welcome",
				Changes: []planner.FieldChange{
					{Field: "topic", OldValue: "old", NewValue: "new"},
				},
			},
		},
	}
	out := planOutput(plan)
	if !strings.Contains(out, `"General/welcome" will be updated`) {
		t.Errorf("expected update line, got:\n%s", out)
	}
	if !strings.Contains(out, `"old" -> "new"`) {
		t.Errorf("expected field change, got:\n%s", out)
	}
}

func TestPrintPlanDeleteAction(t *testing.T) {
	plan := &planner.Plan{
		ToDelete: 1,
		Actions: []planner.Action{
			{Type: planner.ActionDelete, ResourceType: planner.ResourceChannel, Name: "General/old-chat"},
		},
	}
	out := planOutput(plan)
	if !strings.Contains(out, `"General/old-chat" will be deleted`) {
		t.Errorf("expected delete line, got:\n%s", out)
	}
}

func TestPrintPlanWarningOnlyWhenDeletes(t *testing.T) {
	withDelete := &planner.Plan{
		ToDelete: 1,
		Actions:  []planner.Action{{Type: planner.ActionDelete, ResourceType: planner.ResourceChannel, Name: "x"}},
	}
	withCreate := &planner.Plan{
		ToCreate: 1,
		Actions:  []planner.Action{{Type: planner.ActionCreate, ResourceType: planner.ResourceChannel, Name: "y"}},
	}

	if !strings.Contains(planOutput(withDelete), "cannot be undone") {
		t.Error("expected destructive warning when deletes present")
	}
	if strings.Contains(planOutput(withCreate), "cannot be undone") {
		t.Error("unexpected destructive warning when no deletes")
	}
}

func TestPrintPlanDryRunHintWhenChanges(t *testing.T) {
	withChanges := &planner.Plan{
		ToCreate: 1,
		Actions:  []planner.Action{{Type: planner.ActionCreate, ResourceType: planner.ResourceRole, Name: "mod"}},
	}
	noChanges := &planner.Plan{}

	if !strings.Contains(planOutput(withChanges), "--dry-run") {
		t.Error("expected dry-run hint when plan has changes")
	}
	if strings.Contains(planOutput(noChanges), "--dry-run") {
		t.Error("unexpected dry-run hint when plan has no changes")
	}
}

func TestPrintPlanSummaryLine(t *testing.T) {
	plan := &planner.Plan{ToCreate: 2, ToUpdate: 1, ToDelete: 3}
	out := planOutput(plan)
	if !strings.Contains(out, "2 to add") {
		t.Errorf("expected '2 to add' in summary, got:\n%s", out)
	}
	if !strings.Contains(out, "1 to change") {
		t.Errorf("expected '1 to change' in summary, got:\n%s", out)
	}
	if !strings.Contains(out, "3 to destroy") {
		t.Errorf("expected '3 to destroy' in summary, got:\n%s", out)
	}
}

func TestPrintPlanNoChanges(t *testing.T) {
	out := planOutput(&planner.Plan{})
	if strings.Contains(out, "will be created") || strings.Contains(out, "will be deleted") {
		t.Errorf("expected no action lines for empty plan, got:\n%s", out)
	}
}

func TestPrintPlanSortOrder(t *testing.T) {
	// Roles should appear before channels regardless of insertion order.
	plan := &planner.Plan{
		ToCreate: 2,
		Actions: []planner.Action{
			{Type: planner.ActionCreate, ResourceType: planner.ResourceChannel, Name: "General/chat"},
			{Type: planner.ActionCreate, ResourceType: planner.ResourceRole, Name: "admin"},
		},
	}
	out := planOutput(plan)
	roleIdx := strings.Index(out, "role")
	chanIdx := strings.Index(out, "channel")
	if roleIdx == -1 || chanIdx == -1 {
		t.Fatalf("expected both role and channel lines, got:\n%s", out)
	}
	if roleIdx > chanIdx {
		t.Errorf("expected role to appear before channel in output")
	}
}
