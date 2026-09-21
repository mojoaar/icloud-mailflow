package db

import (
	"encoding/json"
	"fmt"
	"strings"
)

type RuleConditionExport struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
}

type RuleActionExport struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// RuleGroupExport is a recursive AND/OR group.
type RuleGroupExport struct {
	Operator   string                `json:"operator"`
	Conditions []RuleConditionExport `json:"conditions,omitempty"`
	Groups     []RuleGroupExport     `json:"groups,omitempty"`
}

// RuleExport is the canonical, portable representation of a rule used by the
// web export/import, the scheduled backup email, and the MCP tools.
type RuleExport struct {
	Name          string                `json:"name"`
	Description   string                `json:"description,omitempty"`
	Priority      int                   `json:"priority"`
	Enabled       bool                  `json:"enabled"`
	Operator      string                `json:"operator,omitempty"`
	ScheduleDays  string                `json:"schedule_days,omitempty"`
	ScheduleStart string                `json:"schedule_start,omitempty"`
	ScheduleEnd   string                `json:"schedule_end,omitempty"`
	Conditions    []RuleConditionExport `json:"conditions,omitempty"`
	Groups        []RuleGroupExport     `json:"groups,omitempty"`
	Actions       []RuleActionExport    `json:"actions,omitempty"`
}

// RulesExport is the {"rules":[...]} envelope written by MarshalExport.
type RulesExport struct {
	Rules []RuleExport `json:"rules"`
}

// ruleImport mirrors RuleExport but keeps "enabled" optional so a missing value
// defaults to true (older backups omitted it).
type ruleImport struct {
	RuleExport
	Enabled *bool `json:"enabled"`
}

type rulesImport struct {
	Rules []ruleImport `json:"rules"`
}

func (ri ruleImport) toExport() RuleExport {
	re := ri.RuleExport
	if ri.Enabled == nil {
		re.Enabled = true
	} else {
		re.Enabled = *ri.Enabled
	}
	return re
}

// Export returns all user rules (excluding the built-in catch-all) in canonical
// portable form.
func (r *RulesRepo) Export() ([]RuleExport, error) {
	rules, err := r.List()
	if err != nil {
		return nil, err
	}
	out := []RuleExport{}
	for _, rule := range rules {
		if rule.Name == "_catch_all" {
			continue
		}
		out = append(out, ruleToExport(rule))
	}
	return out, nil
}

func ruleToExport(rule Rule) RuleExport {
	re := RuleExport{
		Name:          rule.Name,
		Description:   rule.Description,
		Priority:      rule.Priority,
		Enabled:       rule.Enabled,
		ScheduleDays:  rule.ScheduleDays,
		ScheduleStart: rule.ScheduleStart,
		ScheduleEnd:   rule.ScheduleEnd,
	}
	if len(rule.Groups) == 1 && len(rule.Groups[0].Groups) == 0 {
		re.Operator = rule.Groups[0].Operator
		for _, c := range rule.Groups[0].Conditions {
			re.Conditions = append(re.Conditions, RuleConditionExport{Field: c.Field, Operator: c.Operator, Value: c.Value})
		}
	} else {
		for _, g := range rule.Groups {
			re.Groups = append(re.Groups, groupToExport(g))
		}
	}
	for _, a := range rule.Actions {
		re.Actions = append(re.Actions, RuleActionExport{Type: a.Type, Value: a.Value})
	}
	return re
}

func groupToExport(g ConditionGroup) RuleGroupExport {
	ge := RuleGroupExport{Operator: g.Operator}
	for _, c := range g.Conditions {
		ge.Conditions = append(ge.Conditions, RuleConditionExport{Field: c.Field, Operator: c.Operator, Value: c.Value})
	}
	for _, sub := range g.Groups {
		ge.Groups = append(ge.Groups, groupToExport(sub))
	}
	return ge
}

func groupToRule(g RuleGroupExport) ConditionGroup {
	op := g.Operator
	if op != "AND" && op != "OR" {
		op = "OR"
	}
	out := ConditionGroup{Operator: op}
	for _, c := range g.Conditions {
		out.Conditions = append(out.Conditions, Condition{Field: c.Field, Operator: c.Operator, Value: c.Value})
	}
	for _, sub := range g.Groups {
		out.Groups = append(out.Groups, groupToRule(sub))
	}
	return out
}

// MarshalExport renders rules as the canonical {"rules":[...]} envelope.
func MarshalExport(rules []RuleExport) ([]byte, error) {
	return json.MarshalIndent(RulesExport{Rules: rules}, "", "  ")
}

// ImportOptions controls duplicate handling during import.
type ImportOptions struct {
	// OnDuplicate is "skip" (default), "rename", or "error".
	OnDuplicate string
}

// RuleError describes a validation problem with one rule.
type RuleError struct {
	Index   int    `json:"index"`
	Name    string `json:"name"`
	Message string `json:"message"`
}

// ImportReport summarises an import.
type ImportReport struct {
	Imported int         `json:"imported"`
	Skipped  int         `json:"skipped"`
	Warnings []string    `json:"warnings,omitempty"`
	Errors   []RuleError `json:"errors,omitempty"`
}

// ImportPreview is a non-destructive parse + validation of an import file.
type ImportPreview struct {
	Rules     []RuleExport `json:"rules"`
	Duplicate []bool       `json:"duplicate"`
	Report    ImportReport `json:"report"`
}

func parseRulesExport(data []byte) ([]RuleExport, error) {
	var wrapped rulesImport
	var inputs []ruleImport
	if err := json.Unmarshal(data, &wrapped); err == nil && wrapped.Rules != nil {
		inputs = wrapped.Rules
	} else if err := json.Unmarshal(data, &inputs); err != nil {
		return nil, fmt.Errorf("parse rules: %w", err)
	}
	out := make([]RuleExport, 0, len(inputs))
	for _, in := range inputs {
		out = append(out, in.toExport())
	}
	return out, nil
}

func (r *RulesRepo) existingNames() (map[string]bool, error) {
	rules, err := r.List()
	if err != nil {
		return nil, err
	}
	names := make(map[string]bool, len(rules))
	for _, rule := range rules {
		names[strings.ToLower(strings.TrimSpace(rule.Name))] = true
	}
	return names, nil
}

// PreviewImport parses and validates a file without importing anything.
func (r *RulesRepo) PreviewImport(data []byte) (ImportPreview, error) {
	rules, err := parseRulesExport(data)
	if err != nil {
		return ImportPreview{}, err
	}
	existing, err := r.existingNames()
	if err != nil {
		return ImportPreview{}, err
	}

	preview := ImportPreview{Rules: rules, Duplicate: make([]bool, len(rules))}
	seen := map[string]bool{}
	for i, re := range rules {
		for _, m := range ValidateRuleExport(re) {
			preview.Report.Errors = append(preview.Report.Errors, RuleError{Index: i, Name: re.Name, Message: m})
		}
		n := strings.ToLower(strings.TrimSpace(re.Name))
		preview.Duplicate[i] = existing[n] || seen[n]
		seen[n] = true
	}
	return preview, nil
}

// ImportWithReport validates the whole file (rejecting it if any rule is
// invalid) and imports the rest, applying the duplicate policy.
func (r *RulesRepo) ImportWithReport(data []byte, opts ImportOptions) (ImportReport, error) {
	report := ImportReport{}
	rules, err := parseRulesExport(data)
	if err != nil {
		return report, err
	}

	for i, re := range rules {
		for _, m := range ValidateRuleExport(re) {
			report.Errors = append(report.Errors, RuleError{Index: i, Name: re.Name, Message: m})
		}
	}
	if len(report.Errors) > 0 {
		msgs := make([]string, 0, len(report.Errors))
		for _, e := range report.Errors {
			msgs = append(msgs, fmt.Sprintf("%s: %s", e.Name, e.Message))
		}
		return report, fmt.Errorf("import rejected: %s", strings.Join(msgs, "; "))
	}

	existing, err := r.existingNames()
	if err != nil {
		return report, err
	}
	policy := opts.OnDuplicate
	if policy == "" {
		policy = "skip"
	}

	seen := map[string]bool{}
	for i, re := range rules {
		name := strings.TrimSpace(re.Name)
		key := strings.ToLower(name)
		dup := existing[key] || seen[key]
		if dup {
			switch policy {
			case "error":
				report.Errors = append(report.Errors, RuleError{Index: i, Name: name, Message: "duplicate rule name"})
				continue
			case "rename":
				base := name
				for n := 2; existing[strings.ToLower(name)] || seen[strings.ToLower(name)]; n++ {
					name = fmt.Sprintf("%s (%d)", base, n)
				}
				key = strings.ToLower(name)
			default: // skip
				report.Skipped++
				report.Warnings = append(report.Warnings, fmt.Sprintf("skipped duplicate rule %q", name))
				continue
			}
		}
		if err := r.Create(ruleExportToRule(re, name)); err != nil {
			return report, err
		}
		seen[key] = true
		report.Imported++
	}
	if policy == "error" && len(report.Errors) > 0 {
		return report, fmt.Errorf("import rejected: %d duplicate name(s)", len(report.Errors))
	}
	return report, nil
}

func ruleExportToRule(re RuleExport, name string) *Rule {
	rule := &Rule{
		Name:          name,
		Description:   re.Description,
		Priority:      re.Priority,
		Enabled:       re.Enabled,
		ScheduleDays:  re.ScheduleDays,
		ScheduleStart: re.ScheduleStart,
		ScheduleEnd:   re.ScheduleEnd,
	}
	if len(re.Conditions) > 0 {
		op := re.Operator
		if op != "AND" && op != "OR" {
			op = "OR"
		}
		g := ConditionGroup{Operator: op}
		for _, c := range re.Conditions {
			g.Conditions = append(g.Conditions, Condition{
				Field: c.Field, Operator: c.Operator, Value: c.Value,
			})
		}
		rule.Groups = []ConditionGroup{g}
	} else if len(re.Groups) > 0 {
		for _, g := range re.Groups {
			rule.Groups = append(rule.Groups, groupToRule(g))
		}
	}
	for _, a := range re.Actions {
		rule.Actions = append(rule.Actions, Action{Type: a.Type, Value: a.Value})
	}
	return rule
}

// Import creates rules from the canonical envelope or a bare JSON array. A
// missing "enabled" key defaults to true.
func (r *RulesRepo) Import(data []byte) (int, error) {
	report, err := r.ImportWithReport(data, ImportOptions{})
	if err != nil {
		return report.Imported, err
	}
	return report.Imported, nil
}
