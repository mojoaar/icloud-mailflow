package db

import (
	"encoding/json"
	"fmt"
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
	if len(rule.Groups) > 0 {
		re.Operator = rule.Groups[0].Operator
	}
	for _, g := range rule.Groups {
		for _, c := range g.Conditions {
			re.Conditions = append(re.Conditions, RuleConditionExport{
				Field: c.Field, Operator: c.Operator, Value: c.Value,
			})
		}
	}
	for _, a := range rule.Actions {
		re.Actions = append(re.Actions, RuleActionExport{Type: a.Type, Value: a.Value})
	}
	return re
}

// MarshalExport renders rules as the canonical {"rules":[...]} envelope.
func MarshalExport(rules []RuleExport) ([]byte, error) {
	return json.MarshalIndent(RulesExport{Rules: rules}, "", "  ")
}

// Import creates rules from the canonical envelope or a bare JSON array. A
// missing "enabled" key defaults to true.
func (r *RulesRepo) Import(data []byte) (int, error) {
	var wrapped rulesImport
	var inputs []ruleImport
	if err := json.Unmarshal(data, &wrapped); err == nil && wrapped.Rules != nil {
		inputs = wrapped.Rules
	} else if err := json.Unmarshal(data, &inputs); err != nil {
		return 0, fmt.Errorf("parse rules: %w", err)
	}

	imported := 0
	for _, in := range inputs {
		re := in.toExport()
		rule := &Rule{
			Name:          re.Name,
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
		}
		for _, a := range re.Actions {
			rule.Actions = append(rule.Actions, Action{Type: a.Type, Value: a.Value})
		}
		if err := r.Create(rule); err != nil {
			return imported, err
		}
		imported++
	}
	return imported, nil
}
