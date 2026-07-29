package rules

import (
	"fmt"
	"mime"
	"regexp"
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

	switch c.Operator {
	case "exists":
		return val != "", nil
	case "not_exists":
		return val == "", nil
	case "equals":
		return strings.EqualFold(val, c.Value), nil
	case "not_equals":
		return !strings.EqualFold(val, c.Value), nil
	case "contains":
		return strings.Contains(strings.ToLower(val), strings.ToLower(c.Value)), nil
	case "not_contains":
		return !strings.Contains(strings.ToLower(val), strings.ToLower(c.Value)), nil
	case "starts_with":
		return strings.HasPrefix(strings.ToLower(val), strings.ToLower(c.Value)), nil
	case "ends_with":
		return strings.HasSuffix(strings.ToLower(val), strings.ToLower(c.Value)), nil
	case "matches_regex":
		re, err := regexp.Compile(c.Value)
		if err != nil {
			return false, fmt.Errorf("invalid regex %q: %w", c.Value, err)
		}
		return re.MatchString(val), nil
	default:
		return false, nil
	}
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
		name := decodeMIME(a.Name)
		if name != "" {
			parts[i] = fmt.Sprintf("%s <%s>", name, a.Email)
		} else {
			parts[i] = a.Email
		}
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
