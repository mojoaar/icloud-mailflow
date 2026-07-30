package rules

import (
	"fmt"
	"log/slog"
	"mime"
	"strings"

	"github.com/mojoaar/icloud-mailflow/internal/db"
	"github.com/mojoaar/icloud-mailflow/internal/imap"
)

func Match(rules []db.Rule, msg *imap.Message) (*db.Rule, error) {
	for i := range rules {
		if !rules[i].Enabled {
			continue
		}
		ok, err := Evaluate(&rules[i], msg)
		if err != nil {
			return nil, fmt.Errorf("rule %d (%s): %w", rules[i].ID, rules[i].Name, err)
		}
		if ok {
			return &rules[i], nil
		}
	}
	return nil, nil
}

func Evaluate(rule *db.Rule, msg *imap.Message) (bool, error) {
	if len(rule.Groups) == 0 {
		return true, nil
	}
	for _, group := range rule.Groups {
		ok, err := evalGroup(group, msg)
		if err != nil {
			return false, err
		}
		if !ok {
			return false, nil
		}
	}
	return len(rule.Groups) > 0, nil
}

func evalGroup(g db.ConditionGroup, msg *imap.Message) (bool, error) {
	for _, c := range g.Conditions {
		ok, err := evalCondition(c, msg)
		if err != nil {
			return false, err
		}
		if g.Operator == "AND" && !ok {
			return false, nil
		}
		if g.Operator == "OR" && ok {
			return true, nil
		}
	}
	for _, sub := range g.Groups {
		ok, err := evalGroup(sub, msg)
		if err != nil {
			return false, err
		}
		if g.Operator == "AND" && !ok {
			return false, nil
		}
		if g.Operator == "OR" && ok {
			return true, nil
		}
	}
	if g.Operator == "AND" {
		return true, nil
	}
	return false, nil
}

func evalCondition(c db.Condition, msg *imap.Message) (bool, error) {
	if c.Field == "has_attachment" {
		if c.Operator == "exists" {
			return msg.HasAttach, nil
		}
		if c.Operator == "not_exists" {
			return !msg.HasAttach, nil
		}
	}

	val := getFieldValue(c.Field, msg)

	var ok bool
	switch c.Operator {
	case "exists":
		ok = val != ""
	case "not_exists":
		ok = val == ""
	case "equals":
		ok = strings.EqualFold(val, c.Value)
	case "not_equals":
		ok = !strings.EqualFold(val, c.Value)
	case "contains":
		ok = strings.Contains(strings.ToLower(val), strings.ToLower(c.Value))
	case "not_contains":
		ok = !strings.Contains(strings.ToLower(val), strings.ToLower(c.Value))
	case "starts_with":
		ok = strings.HasPrefix(strings.ToLower(val), strings.ToLower(c.Value))
	case "ends_with":
		ok = strings.HasSuffix(strings.ToLower(val), strings.ToLower(c.Value))
	case "matches_regex":
		if c.CompiledRegex == nil {
			ok = false
		} else {
			ok = c.CompiledRegex.MatchString(val)
		}
	}
	slog.Debug("condition evaluated", "field", c.Field, "op", c.Operator, "expected", c.Value, "actual", val, "match", ok)
	return ok, nil
}

func getFieldValue(field string, msg *imap.Message) string {
	switch {
	case field == "from":
		return addrsToString(msg.From)
	case field == "to":
		return addrsToString(msg.To)
	case field == "cc":
		return addrsToString(msg.Cc)
	case field == "subject":
		return decodeMIME(msg.Subject)
	case field == "body":
		return ""
	case strings.HasPrefix(field, "header:"):
		return ""
	}
	return ""
}

func addrsToString(addrs []imap.Address) string {
	if len(addrs) == 0 {
		return ""
	}
	parts := make([]string, len(addrs))
	for i, a := range addrs {
		parts[i] = a.Email
	}
	return strings.Join(parts, ", ")
}

func decodeMIME(s string) string {
	dec := new(mime.WordDecoder)
	decoded, err := dec.DecodeHeader(s)
	if err != nil {
		return s
	}
	return decoded
}
