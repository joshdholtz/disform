package cmd

import (
	"strings"
	"testing"

	"github.com/joshholtz/disform/internal/planner"
)

func TestConfirmApplyYes(t *testing.T) {
	plan := &planner.Plan{ToCreate: 1, Actions: []planner.Action{
		{Type: planner.ActionCreate, ResourceType: planner.ResourceRole, Name: "admin"},
	}}
	ok, err := confirmApply(strings.NewReader("yes\n"), plan)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("expected confirmed=true for 'yes'")
	}
}

func TestConfirmApplyNo(t *testing.T) {
	plan := &planner.Plan{ToCreate: 1, Actions: []planner.Action{
		{Type: planner.ActionCreate, ResourceType: planner.ResourceRole, Name: "admin"},
	}}
	ok, err := confirmApply(strings.NewReader("no\n"), plan)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("expected confirmed=false for 'no'")
	}
}

func TestConfirmApplyEmptyInput(t *testing.T) {
	plan := &planner.Plan{ToCreate: 1, Actions: []planner.Action{
		{Type: planner.ActionCreate, ResourceType: planner.ResourceRole, Name: "admin"},
	}}
	ok, err := confirmApply(strings.NewReader("\n"), plan)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("expected confirmed=false for empty input")
	}
}

func TestConfirmApplyWithDeletesCorrectCount(t *testing.T) {
	plan := &planner.Plan{
		ToDelete: 2,
		Actions: []planner.Action{
			{Type: planner.ActionDelete, ResourceType: planner.ResourceChannel, Name: "General/chat"},
			{Type: planner.ActionDelete, ResourceType: planner.ResourceRole, Name: "mod"},
		},
	}
	// First prompt: "yes", second prompt: "2"
	ok, err := confirmApply(strings.NewReader("yes\n2\n"), plan)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("expected confirmed=true when correct count provided")
	}
}

func TestConfirmApplyWithDeletesWrongCount(t *testing.T) {
	plan := &planner.Plan{
		ToDelete: 2,
		Actions: []planner.Action{
			{Type: planner.ActionDelete, ResourceType: planner.ResourceChannel, Name: "General/chat"},
			{Type: planner.ActionDelete, ResourceType: planner.ResourceRole, Name: "mod"},
		},
	}
	ok, err := confirmApply(strings.NewReader("yes\n1\n"), plan)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("expected confirmed=false when wrong count provided")
	}
}

func TestConfirmApplyWithDeletesNonNumeric(t *testing.T) {
	plan := &planner.Plan{
		ToDelete: 1,
		Actions: []planner.Action{
			{Type: planner.ActionDelete, ResourceType: planner.ResourceChannel, Name: "General/chat"},
		},
	}
	ok, err := confirmApply(strings.NewReader("yes\nyes\n"), plan)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("expected confirmed=false when 'yes' typed instead of count")
	}
}

func TestConfirmApplyWithDeletesFirstPromptNo(t *testing.T) {
	plan := &planner.Plan{
		ToDelete: 1,
		Actions: []planner.Action{
			{Type: planner.ActionDelete, ResourceType: planner.ResourceChannel, Name: "General/chat"},
		},
	}
	// Cancels at the first prompt — second prompt never reached.
	ok, err := confirmApply(strings.NewReader("no\n"), plan)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("expected confirmed=false when first prompt is 'no'")
	}
}

func TestConfirmApplyNoDeletesSkipsSecondPrompt(t *testing.T) {
	// Plan with only creates — no second prompt should be needed.
	plan := &planner.Plan{
		ToCreate: 1,
		Actions: []planner.Action{
			{Type: planner.ActionCreate, ResourceType: planner.ResourceRole, Name: "admin"},
		},
	}
	// Only provide one line — if a second prompt were issued it would EOF and error.
	ok, err := confirmApply(strings.NewReader("yes\n"), plan)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("expected confirmed=true with no deletes")
	}
}
