package db

import (
	"testing"
)

func TestRulesCreateAndGet(t *testing.T) {
	db := openTestDB(t)
	repo := NewRulesRepo(db)

	rule := &Rule{
		Name:     "Test Rule",
		Priority: 1,
		Enabled:  true,
		Groups: []ConditionGroup{
			{
				Operator: "AND",
				Conditions: []Condition{
					{Field: "from", Operator: "equals", Value: "alice@example.com"},
					{Field: "subject", Operator: "contains", Value: "invoice"},
				},
			},
		},
		Actions: []Action{
			{Type: "move_to_folder", Value: "Bills"},
		},
	}

	if err := repo.Create(rule); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if rule.ID == 0 {
		t.Error("ID should be set after create")
	}

	got, err := repo.Get(rule.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "Test Rule" {
		t.Errorf("Name = %q", got.Name)
	}
	if len(got.Groups) != 1 {
		t.Fatalf("Groups len = %d, want 1", len(got.Groups))
	}
	if len(got.Groups[0].Conditions) != 2 {
		t.Errorf("Conditions len = %d, want 2", len(got.Groups[0].Conditions))
	}
	if len(got.Actions) != 1 {
		t.Errorf("Actions len = %d, want 1", len(got.Actions))
	}
}

func TestRulesList(t *testing.T) {
	db := openTestDB(t)
	repo := NewRulesRepo(db)

	repo.Create(&Rule{Name: "Second", Priority: 2, Enabled: true})
	repo.Create(&Rule{Name: "First", Priority: 1, Enabled: true})

	rules, err := repo.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(rules) != 2 {
		t.Fatalf("len = %d, want 2", len(rules))
	}
	if rules[0].Name != "First" {
		t.Errorf("first rule = %q, want First (priority 1)", rules[0].Name)
	}
}

func TestRulesUpdate(t *testing.T) {
	db := openTestDB(t)
	repo := NewRulesRepo(db)

	rule := &Rule{Name: "Original", Priority: 0, Enabled: true}
	repo.Create(rule)

	rule.Name = "Updated"
	rule.Enabled = false
	rule.Groups = []ConditionGroup{
		{Operator: "OR", Conditions: []Condition{{Field: "to", Operator: "equals", Value: "b@c.com"}}},
	}
	rule.Actions = []Action{{Type: "move_to_folder", Value: "Archive"}}

	if err := repo.Update(rule); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, _ := repo.Get(rule.ID)
	if got.Name != "Updated" {
		t.Errorf("Name = %q", got.Name)
	}
	if got.Enabled {
		t.Error("should be disabled")
	}
	if len(got.Groups) != 1 || got.Groups[0].Operator != "OR" {
		t.Error("groups not updated correctly")
	}
	if len(got.Actions) != 1 || got.Actions[0].Value != "Archive" {
		t.Error("actions not updated correctly")
	}
}

func TestRulesDelete(t *testing.T) {
	db := openTestDB(t)
	repo := NewRulesRepo(db)

	repo.Create(&Rule{Name: "DeleteMe", Priority: 0, Enabled: true})
	repo.Create(&Rule{Name: "KeepMe", Priority: 1, Enabled: true})

	rules, _ := repo.List()
	if err := repo.Delete(rules[0].ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	remaining, _ := repo.List()
	if len(remaining) != 1 {
		t.Errorf("len = %d, want 1", len(remaining))
	}
	if remaining[0].Name != "KeepMe" {
		t.Errorf("wrong rule deleted, got %q", remaining[0].Name)
	}
}

func TestRulesReorder(t *testing.T) {
	db := openTestDB(t)
	repo := NewRulesRepo(db)

	r1 := &Rule{Name: "A", Priority: 0, Enabled: true}
	r2 := &Rule{Name: "B", Priority: 1, Enabled: true}
	r3 := &Rule{Name: "C", Priority: 2, Enabled: true}
	repo.Create(r1)
	repo.Create(r2)
	repo.Create(r3)

	if err := repo.Reorder([]int64{r3.ID, r1.ID, r2.ID}); err != nil {
		t.Fatalf("Reorder: %v", err)
	}

	rules, _ := repo.List()
	if rules[0].Name != "C" || rules[1].Name != "A" || rules[2].Name != "B" {
		t.Errorf("wrong order: %v", []string{rules[0].Name, rules[1].Name, rules[2].Name})
	}
}

func TestRulesEnsureCatchAllWhenMissing(t *testing.T) {
	db := openTestDB(t)
	repo := NewRulesRepo(db)

	if err := repo.EnsureCatchAll(); err != nil {
		t.Fatalf("EnsureCatchAll: %v", err)
	}

	rules, _ := repo.List()
	if len(rules) != 1 {
		t.Fatalf("len = %d, want 1", len(rules))
	}
	if rules[0].Name != "_catch_all" {
		t.Errorf("Name = %q, want _catch_all", rules[0].Name)
	}
	if len(rules[0].Groups) != 0 {
		t.Error("catch-all should have no condition groups")
	}
}

func TestRulesEnsureCatchAllIdempotent(t *testing.T) {
	db := openTestDB(t)
	repo := NewRulesRepo(db)

	repo.EnsureCatchAll()
	repo.EnsureCatchAll()
	repo.EnsureCatchAll()

	rules, _ := repo.List()
	if len(rules) != 1 {
		t.Errorf("len = %d, want 1", len(rules))
	}
}

func TestBuildGroupTreeFlat(t *testing.T) {
	all := []ConditionGroup{
		{ID: 1, Operator: "AND", ParentID: nil},
		{ID: 2, Operator: "OR", ParentID: nil},
	}

	result := buildGroupTree(all, nil)
	if len(result) != 2 {
		t.Errorf("len = %d, want 2", len(result))
	}
}

func TestBuildGroupTreeNested(t *testing.T) {
	parentID := int64(1)
	all := []ConditionGroup{
		{ID: 1, Operator: "AND", ParentID: nil},
		{ID: 2, Operator: "OR", ParentID: &parentID},
	}

	result := buildGroupTree(all, nil)
	if len(result) != 1 {
		t.Fatalf("len = %d, want 1", len(result))
	}
	if len(result[0].Groups) != 1 {
		t.Errorf("nested groups len = %d, want 1", len(result[0].Groups))
	}
	if result[0].Groups[0].ID != 2 {
		t.Errorf("nested group ID = %d, want 2", result[0].Groups[0].ID)
	}
}

func TestRulesGetNotFound(t *testing.T) {
	db := openTestDB(t)
	repo := NewRulesRepo(db)

	_, err := repo.Get(9999)
	if err == nil {
		t.Error("Get should return error for missing rule")
	}
}
