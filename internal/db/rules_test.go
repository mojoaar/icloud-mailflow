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

func TestRulesDeleteCascade(t *testing.T) {
	db := openTestDB(t)
	repo := NewRulesRepo(db)

	r := &Rule{
		Name:    "test cascade",
		Groups:  []ConditionGroup{{Operator: "AND", Conditions: []Condition{{Field: "subject", Operator: "contains", Value: "hello"}}}},
		Actions: []Action{{Type: "move_to_folder", Value: "Test"}},
	}
	if err := repo.Create(r); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := repo.Delete(r.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	var orphaned int
	if err := db.QueryRow(`SELECT COUNT(*) FROM condition_groups WHERE rule_id = ?`, r.ID).Scan(&orphaned); err != nil {
		t.Fatalf("count groups: %v", err)
	}
	if orphaned != 0 {
		t.Errorf("orphaned condition_groups = %d, want 0", orphaned)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM conditions WHERE group_id NOT IN (SELECT id FROM condition_groups)`).Scan(&orphaned); err != nil {
		t.Fatalf("count conditions: %v", err)
	}
	if orphaned != 0 {
		t.Errorf("orphaned conditions = %d, want 0", orphaned)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM actions WHERE rule_id = ?`, r.ID).Scan(&orphaned); err != nil {
		t.Fatalf("count actions: %v", err)
	}
	if orphaned != 0 {
		t.Errorf("orphaned actions = %d, want 0", orphaned)
	}
}

func TestRulesScheduleRoundTrip(t *testing.T) {
	db := openTestDB(t)
	repo := NewRulesRepo(db)

	rule := &Rule{
		Name:          "Scheduled Rule",
		Priority:      1,
		Enabled:       true,
		ScheduleDays:  "mon,wed,fri",
		ScheduleStart: "09:00",
		ScheduleEnd:   "17:00",
	}
	if err := repo.Create(rule); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, _ := repo.Get(rule.ID)
	if got.ScheduleDays != "mon,wed,fri" {
		t.Errorf("ScheduleDays = %q, want mon,wed,fri", got.ScheduleDays)
	}
	if got.ScheduleStart != "09:00" {
		t.Errorf("ScheduleStart = %q, want 09:00", got.ScheduleStart)
	}
	if got.ScheduleEnd != "17:00" {
		t.Errorf("ScheduleEnd = %q, want 17:00", got.ScheduleEnd)
	}

	got.ScheduleDays = "tue,thu"
	got.ScheduleStart = "10:00"
	got.ScheduleEnd = "14:00"
	if err := repo.Update(got); err != nil {
		t.Fatalf("Update: %v", err)
	}

	updated, _ := repo.Get(rule.ID)
	if updated.ScheduleDays != "tue,thu" {
		t.Errorf("updated ScheduleDays = %q, want tue,thu", updated.ScheduleDays)
	}
	if updated.ScheduleStart != "10:00" {
		t.Errorf("updated ScheduleStart = %q, want 10:00", updated.ScheduleStart)
	}
	if updated.ScheduleEnd != "14:00" {
		t.Errorf("updated ScheduleEnd = %q, want 14:00", updated.ScheduleEnd)
	}

	rules, _ := repo.List()
	for _, r := range rules {
		if r.ID == rule.ID {
			if r.ScheduleDays != "tue,thu" {
				t.Errorf("List ScheduleDays = %q, want tue,thu", r.ScheduleDays)
			}
			return
		}
	}
	t.Error("rule not found in List")
}

func TestReorderIgnoresInvalidIDs(t *testing.T) {
	d := NewTestDB(t)
	repo := NewRulesRepo(d)
	r1 := &Rule{Name: "a", Enabled: true}
	r2 := &Rule{Name: "b", Enabled: true}
	if err := repo.Create(r1); err != nil {
		t.Fatalf("Create r1: %v", err)
	}
	if err := repo.Create(r2); err != nil {
		t.Fatalf("Create r2: %v", err)
	}
	if err := repo.Reorder([]int64{r2.ID, 0, r1.ID}); err != nil {
		t.Fatalf("Reorder: %v", err)
	}
	got1, _ := repo.Get(r1.ID)
	got2, _ := repo.Get(r2.ID)
	if got2.Priority != 0 || got1.Priority != 1 {
		t.Errorf("priorities: r2=%d r1=%d, want 0 and 1", got2.Priority, got1.Priority)
	}
}

func TestListHydratesMultipleRules(t *testing.T) {
	d := NewTestDB(t)
	repo := NewRulesRepo(d)
	r1 := &Rule{
		Name: "one", Enabled: true,
		Groups: []ConditionGroup{{Operator: "AND", Conditions: []Condition{
			{Field: "from", Operator: "contains", Value: "a"},
			{Field: "subject", Operator: "equals", Value: "s"},
		}}},
		Actions: []Action{{Type: "move_to_folder", Value: "A"}, {Type: "mark_as_read"}},
	}
	r2 := &Rule{
		Name: "two", Enabled: true,
		Groups: []ConditionGroup{{Operator: "OR", Conditions: []Condition{
			{Field: "body", Operator: "contains", Value: "b"},
		}}},
		Actions: []Action{{Type: "forward", Value: "x@y.com"}},
	}
	for _, r := range []*Rule{r1, r2} {
		if err := repo.Create(r); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	rules, err := repo.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	byName := map[string]Rule{}
	for _, r := range rules {
		byName[r.Name] = r
	}
	one := byName["one"]
	if len(one.Groups) != 1 || one.Groups[0].Operator != "AND" || len(one.Groups[0].Conditions) != 2 {
		t.Errorf("rule one groups = %+v", one.Groups)
	}
	if len(one.Actions) != 2 || one.Actions[0].Type != "move_to_folder" || one.Actions[1].Type != "mark_as_read" {
		t.Errorf("rule one actions = %+v", one.Actions)
	}
	two := byName["two"]
	if len(two.Groups) != 1 || two.Groups[0].Operator != "OR" || len(two.Groups[0].Conditions) != 1 {
		t.Errorf("rule two groups = %+v", two.Groups)
	}
	if len(two.Actions) != 1 || two.Actions[0].Type != "forward" {
		t.Errorf("rule two actions = %+v", two.Actions)
	}
}

func TestEnsureCatchAllIdempotent(t *testing.T) {
	d := NewTestDB(t)
	repo := NewRulesRepo(d)
	if err := repo.EnsureCatchAll(); err != nil {
		t.Fatalf("EnsureCatchAll: %v", err)
	}
	rules, _ := repo.List()
	var id int64
	for _, r := range rules {
		if r.Name == "_catch_all" {
			id = r.ID
		}
	}
	if id == 0 {
		t.Fatal("catch-all not created")
	}
	first, _ := repo.Get(id)
	if len(first.Actions) != 1 {
		t.Fatalf("actions = %d, want 1", len(first.Actions))
	}

	if err := repo.EnsureCatchAll(); err != nil {
		t.Fatalf("EnsureCatchAll 2: %v", err)
	}
	second, _ := repo.Get(id)
	if len(second.Actions) != 1 {
		t.Errorf("actions = %d, want 1 after second call", len(second.Actions))
	}
	if second.Actions[0].ID != first.Actions[0].ID {
		t.Error("catch-all action row was churned")
	}
}
