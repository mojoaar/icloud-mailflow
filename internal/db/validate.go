package db

import (
	"fmt"
	"net/mail"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var validConditionFields = map[string]bool{
	"from": true, "to": true, "cc": true, "subject": true, "body": true,
	"content_type": true, "has_attachment": true,
}

var validOperators = map[string]bool{
	"equals": true, "not_equals": true, "contains": true, "not_contains": true,
	"starts_with": true, "ends_with": true, "matches_regex": true,
	"exists": true, "not_exists": true,
	"older_than": true, "newer_than": true, "before": true, "after": true,
}

var validActionTypes = map[string]bool{
	"move_to_folder": true, "mark_as_read": true, "mark_as_unread": true,
	"set_flag": true, "remove_flag": true, "auto_reply": true,
	"forward": true, "delete": true, "webhook": true,
}

var validScheduleDays = map[string]bool{
	"mon": true, "tue": true, "wed": true, "thu": true, "fri": true, "sat": true, "sun": true,
}

var validGroupOperators = map[string]bool{"AND": true, "OR": true}

var daysRe = regexp.MustCompile(`^(\d+) days$`)

// ValidateRuleExport returns human-readable problems with a portable rule. An
// empty slice means the rule is valid.
func ValidateRuleExport(re RuleExport) []string {
	var errs []string
	var total int
	if strings.TrimSpace(re.Name) == "" {
		errs = append(errs, "name is required")
	}
	if re.Operator != "" && !validGroupOperators[re.Operator] {
		errs = append(errs, fmt.Sprintf("unknown group operator %q", re.Operator))
	}
	if len(re.Groups) > 0 {
		for _, g := range re.Groups {
			validateGroupExport(g, 1, &total, &errs)
		}
	} else {
		for i, c := range re.Conditions {
			errs = append(errs, validateCondition(i, c)...)
		}
		total = len(re.Conditions)
	}
	if total == 0 {
		// No conditions is valid: the rule matches every message (optionally
		// limited by its schedule), like the built-in catch-all.
	} else if total > 50 {
		errs = append(errs, "too many conditions (max 50)")
	}
	if msg := validateSchedule(re); msg != "" {
		errs = append(errs, msg)
	}
	for i, a := range re.Actions {
		errs = append(errs, validateAction(i, a)...)
	}
	return errs
}

func validateGroupExport(g RuleGroupExport, depth int, total *int, errs *[]string) {
	if depth > 5 {
		*errs = append(*errs, "group nesting too deep (max 5)")
		return
	}
	if g.Operator != "" && !validGroupOperators[g.Operator] {
		*errs = append(*errs, fmt.Sprintf("unknown group operator %q", g.Operator))
	}
	for i, c := range g.Conditions {
		*total += 1
		*errs = append(*errs, validateCondition(i, c)...)
	}
	for _, sub := range g.Groups {
		validateGroupExport(sub, depth+1, total, errs)
	}
}

func validateCondition(i int, c RuleConditionExport) []string {
	var errs []string
	if strings.HasPrefix(c.Field, "header:") {
		if strings.TrimSpace(strings.TrimPrefix(c.Field, "header:")) == "" {
			errs = append(errs, fmt.Sprintf("condition %d: header name is required", i))
		}
	} else if !validConditionFields[c.Field] {
		errs = append(errs, fmt.Sprintf("condition %d: unknown field %q", i, c.Field))
	}
	if !validOperators[c.Operator] {
		errs = append(errs, fmt.Sprintf("condition %d: unknown operator %q", i, c.Operator))
		return errs
	}
	switch c.Operator {
	case "matches_regex":
		if _, err := regexp.Compile(c.Value); err != nil {
			errs = append(errs, fmt.Sprintf("condition %d: invalid regex: %v", i, err))
		}
	case "older_than", "newer_than":
		m := daysRe.FindStringSubmatch(c.Value)
		if m == nil {
			errs = append(errs, fmt.Sprintf("condition %d: expected 'N days'", i))
		} else if n, _ := strconv.Atoi(m[1]); n <= 0 {
			errs = append(errs, fmt.Sprintf("condition %d: day count must be positive", i))
		}
	case "before", "after":
		if _, err := time.Parse("2006-01-02", c.Value); err != nil {
			errs = append(errs, fmt.Sprintf("condition %d: expected an ISO date (YYYY-MM-DD)", i))
		}
	}
	return errs
}

func validateAction(i int, a RuleActionExport) []string {
	if !validActionTypes[a.Type] {
		return []string{fmt.Sprintf("action %d: unknown type %q", i, a.Type)}
	}
	switch a.Type {
	case "move_to_folder":
		if strings.TrimSpace(a.Value) == "" {
			return []string{fmt.Sprintf("action %d: folder is required", i)}
		}
	case "forward":
		if _, err := mail.ParseAddress(a.Value); err != nil {
			return []string{fmt.Sprintf("action %d: invalid email %q", i, a.Value)}
		}
	case "webhook":
		u, err := url.Parse(a.Value)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
			return []string{fmt.Sprintf("action %d: webhook must be an http(s) URL", i)}
		}
	case "set_flag", "remove_flag", "auto_reply":
		if strings.TrimSpace(a.Value) == "" {
			return []string{fmt.Sprintf("action %d: value is required", i)}
		}
	}
	return nil
}

// ValidateRule validates a stored rule exactly as ValidateRuleExport does.
func ValidateRule(rule *Rule) []string {
	return ValidateRuleExport(ruleToExport(*rule))
}

func validateSchedule(re RuleExport) string {
	for _, d := range strings.Split(re.ScheduleDays, ",") {
		d = strings.ToLower(strings.TrimSpace(d))
		if d == "" {
			continue
		}
		if !validScheduleDays[d] {
			return fmt.Sprintf("unknown schedule day %q", d)
		}
	}
	for _, t := range []string{re.ScheduleStart, re.ScheduleEnd} {
		if t == "" {
			continue
		}
		if _, err := time.Parse("15:04", t); err != nil {
			return fmt.Sprintf("invalid schedule time %q (expected HH:MM)", t)
		}
	}
	return ""
}
