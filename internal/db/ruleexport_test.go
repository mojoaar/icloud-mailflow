package db

import "testing"

func TestRuleExportImportRoundTrip(t *testing.T) {
	src := NewTestDB(t)
	srcRepo := NewRulesRepo(src)
	rule := &Rule{
		Name:          "Scheduled",
		Description:   "d",
		Priority:      7,
		Enabled:       false,
		ScheduleDays:  "mon,wed",
		ScheduleStart: "09:00",
		ScheduleEnd:   "17:00",
		Groups: []ConditionGroup{
			{Operator: "AND", Conditions: []Condition{{Field: "from", Operator: "contains", Value: "@x.com"}}},
		},
		Actions: []Action{{Type: "move_to_folder", Value: "Archive"}},
	}
	if err := srcRepo.Create(rule); err != nil {
		t.Fatalf("Create: %v", err)
	}
	exported, err := srcRepo.Export()
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if len(exported) != 1 {
		t.Fatalf("exported %d rules, want 1", len(exported))
	}
	data, err := MarshalExport(exported)
	if err != nil {
		t.Fatalf("MarshalExport: %v", err)
	}

	dst := NewTestDB(t)
	dstRepo := NewRulesRepo(dst)
	n, err := dstRepo.Import(data)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if n != 1 {
		t.Fatalf("imported %d, want 1", n)
	}
	rules, err := dstRepo.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("rules = %d, want 1", len(rules))
	}
	got := rules[0]
	if got.Name != "Scheduled" || got.Priority != 7 || got.Enabled {
		t.Errorf("meta not preserved: %+v", got)
	}
	if got.ScheduleDays != "mon,wed" || got.ScheduleStart != "09:00" || got.ScheduleEnd != "17:00" {
		t.Errorf("schedule not preserved: %q %q %q", got.ScheduleDays, got.ScheduleStart, got.ScheduleEnd)
	}
	if len(got.Groups) != 1 || got.Groups[0].Operator != "AND" || len(got.Groups[0].Conditions) != 1 {
		t.Errorf("conditions not preserved: %+v", got.Groups)
	}
	if len(got.Actions) != 1 || got.Actions[0].Type != "move_to_folder" || got.Actions[0].Value != "Archive" {
		t.Errorf("actions not preserved: %+v", got.Actions)
	}
}

func TestRuleImportBareArrayDefaultsEnabled(t *testing.T) {
	repo := NewRulesRepo(NewTestDB(t))
	bare := []byte(`[{"name":"bare","conditions":[{"field":"from","operator":"contains","value":"a"}],"actions":[{"type":"mark_as_read","value":""}]}]`)
	n, err := repo.Import(bare)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if n != 1 {
		t.Fatalf("imported %d, want 1", n)
	}
	rules, _ := repo.List()
	if len(rules) != 1 {
		t.Fatalf("rules = %d, want 1", len(rules))
	}
	if !rules[0].Enabled {
		t.Error("a missing 'enabled' key should default to true")
	}
}

func TestRuleExportExcludesCatchAll(t *testing.T) {
	repo := NewRulesRepo(NewTestDB(t))
	if err := repo.EnsureCatchAll(); err != nil {
		t.Fatalf("EnsureCatchAll: %v", err)
	}
	exported, err := repo.Export()
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	for _, e := range exported {
		if e.Name == "_catch_all" {
			t.Error("catch-all must not be exported")
		}
	}
}
