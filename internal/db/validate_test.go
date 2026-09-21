package db

import (
	"strings"
	"testing"
)

func validExport(name string) RuleExport {
	return RuleExport{
		Name:     name,
		Priority: 1,
		Enabled:  true,
		Operator: "AND",
		Conditions: []RuleConditionExport{
			{Field: "from", Operator: "contains", Value: "@x.com"},
		},
		Actions: []RuleActionExport{{Type: "move_to_folder", Value: "Archive"}},
	}
}

func TestValidateRuleExport(t *testing.T) {
	if msgs := ValidateRuleExport(validExport("ok")); len(msgs) != 0 {
		t.Errorf("valid rule reported errors: %v", msgs)
	}

	cases := []struct {
		name string
		mut  func(*RuleExport)
	}{
		{"no name", func(r *RuleExport) { r.Name = "" }},
		{"no conditions", func(r *RuleExport) { r.Conditions = nil }},
		{"bad field", func(r *RuleExport) { r.Conditions[0].Field = "nope" }},
		{"bad operator", func(r *RuleExport) { r.Conditions[0].Operator = "nope" }},
		{"bad regex", func(r *RuleExport) { r.Conditions[0].Operator = "matches_regex"; r.Conditions[0].Value = "(" }},
		{"bad days", func(r *RuleExport) { r.Conditions[0].Operator = "older_than"; r.Conditions[0].Value = "seven days" }},
		{"bad date", func(r *RuleExport) { r.Conditions[0].Operator = "before"; r.Conditions[0].Value = "01/01/2020" }},
		{"empty header", func(r *RuleExport) { r.Conditions[0].Field = "header:" }},
		{"bad group op", func(r *RuleExport) { r.Operator = "XOR" }},
		{"bad action", func(r *RuleExport) { r.Actions[0].Type = "explode" }},
		{"empty folder", func(r *RuleExport) { r.Actions[0].Value = "" }},
		{"bad forward", func(r *RuleExport) { r.Actions[0] = RuleActionExport{Type: "forward", Value: "not-an-email"} }},
		{"bad webhook", func(r *RuleExport) { r.Actions[0] = RuleActionExport{Type: "webhook", Value: "ftp://x"} }},
		{"bad day", func(r *RuleExport) { r.ScheduleDays = "funday" }},
		{"bad time", func(r *RuleExport) { r.ScheduleStart = "9am" }},
	}
	for _, tc := range cases {
		re := validExport("x")
		tc.mut(&re)
		if msgs := ValidateRuleExport(re); len(msgs) == 0 {
			t.Errorf("%s: expected a validation error", tc.name)
		}
	}
}

func TestImportRejectsInvalidFile(t *testing.T) {
	repo := NewRulesRepo(NewTestDB(t))
	data := []byte(`{"rules":[{"name":"bad","operator":"XOR","conditions":[{"field":"from","operator":"contains","value":"a"}],"actions":[]}]}`)
	_, err := repo.Import(data)
	if err == nil {
		t.Fatal("expected import to be rejected")
	}
	if rules, _ := repo.List(); len(rules) != 0 {
		t.Errorf("invalid import must not create rules, got %d", len(rules))
	}
}

func TestImportSkipsDuplicates(t *testing.T) {
	repo := NewRulesRepo(NewTestDB(t))
	if err := repo.Create(&Rule{Name: "Dup", Enabled: true}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	data, _ := MarshalExport([]RuleExport{validExport("Dup"), validExport("Fresh")})

	report, err := repo.ImportWithReport(data, ImportOptions{})
	if err != nil {
		t.Fatalf("ImportWithReport: %v", err)
	}
	if report.Imported != 1 || report.Skipped != 1 {
		t.Errorf("imported=%d skipped=%d, want 1 and 1", report.Imported, report.Skipped)
	}
	if len(report.Warnings) == 0 || !strings.Contains(report.Warnings[0], "Dup") {
		t.Errorf("expected a duplicate warning, got %v", report.Warnings)
	}
}

func TestPreviewImportFlagsDuplicatesAndErrors(t *testing.T) {
	repo := NewRulesRepo(NewTestDB(t))
	if err := repo.Create(&Rule{Name: "Dup", Enabled: true}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	data, _ := MarshalExport([]RuleExport{validExport("Dup"), validExport("New")})

	preview, err := repo.PreviewImport(data)
	if err != nil {
		t.Fatalf("PreviewImport: %v", err)
	}
	if len(preview.Rules) != 2 || !preview.Duplicate[0] || preview.Duplicate[1] {
		t.Errorf("duplicate flags = %v, want [true false]", preview.Duplicate)
	}
	if len(preview.Report.Errors) != 0 {
		t.Errorf("unexpected errors: %v", preview.Report.Errors)
	}

	bad, _ := MarshalExport([]RuleExport{{Name: "broken", Conditions: nil}})
	pbad, err := repo.PreviewImport(bad)
	if err != nil {
		t.Fatalf("PreviewImport bad: %v", err)
	}
	if len(pbad.Report.Errors) == 0 {
		t.Error("expected validation errors for a rule with no conditions")
	}
}
