package rules

import (
	"fmt"
	"log/slog"
	"mime"
	"strings"
	"time"

	"github.com/mojoaar/icloud-mailflow/internal/db"
	"github.com/mojoaar/icloud-mailflow/internal/imap"
)

type msgExtras struct {
	body    string
	headers map[string]string
}

func scanNeeds(g db.ConditionGroup) (needsBody bool, needsHeaders []string) {
	for _, c := range g.Conditions {
		if c.Field == "body" {
			needsBody = true
		}
		if strings.HasPrefix(c.Field, "header:") {
			needsHeaders = append(needsHeaders, strings.TrimPrefix(c.Field, "header:"))
		}
	}
	for _, sub := range g.Groups {
		b, hs := scanNeeds(sub)
		if b {
			needsBody = true
		}
		needsHeaders = append(needsHeaders, hs...)
	}
	return
}

func Match(rules []db.Rule, msg *imap.Message, client imap.Client) (*db.Rule, error) {
	for i := range rules {
		if !rules[i].Enabled {
			continue
		}
		ok, err := Evaluate(&rules[i], msg, client)
		if err != nil {
			return nil, fmt.Errorf("rule %d (%s): %w", rules[i].ID, rules[i].Name, err)
		}
		if ok {
			return &rules[i], nil
		}
	}
	return nil, nil
}

func Evaluate(rule *db.Rule, msg *imap.Message, client imap.Client) (bool, error) {
	if len(rule.Groups) == 0 {
		return true, nil
	}
	extras := &msgExtras{headers: map[string]string{}}
	for _, group := range rule.Groups {
		needsBody, needsHeaders := scanNeeds(group)
		if needsBody && client != nil && extras.body == "" {
			body, err := client.FetchMessageBody(msg.UID)
			if err != nil {
				slog.Warn("failed to fetch body", "uid", msg.UID, "error", err)
			} else {
				extras.body = body
			}
		}
		if client != nil {
			for _, h := range needsHeaders {
				if _, ok := extras.headers[h]; ok {
					continue
				}
				v, err := client.FetchMessageHeader(msg.UID, h)
				if err != nil {
					slog.Warn("failed to fetch header", "uid", msg.UID, "header", h, "error", err)
				} else {
					extras.headers[h] = v
				}
			}
		}
	}
	for _, group := range rule.Groups {
		ok, err := evalGroupWithExtras(group, msg, extras)
		if err != nil {
			return false, err
		}
		if !ok {
			return false, nil
		}
	}
	return len(rule.Groups) > 0, nil
}

func evalGroupWithExtras(g db.ConditionGroup, msg *imap.Message, extras *msgExtras) (bool, error) {
	for _, c := range g.Conditions {
		ok, err := evalConditionWithExtras(c, msg, extras)
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
		ok, err := evalGroupWithExtras(sub, msg, extras)
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

func evalConditionWithExtras(c db.Condition, msg *imap.Message, extras *msgExtras) (bool, error) {
	if c.Field == "has_attachment" {
		if c.Operator == "exists" {
			return msg.HasAttach, nil
		}
		if c.Operator == "not_exists" {
			return !msg.HasAttach, nil
		}
	}

	switch c.Operator {
	case "older_than":
		days, err := parseDays(c.Value)
		if err != nil {
			return false, err
		}
		if msg.Date.IsZero() {
			return false, nil
		}
		return time.Since(msg.Date) > time.Duration(days)*24*time.Hour, nil
	case "newer_than":
		days, err := parseDays(c.Value)
		if err != nil {
			return false, err
		}
		if msg.Date.IsZero() {
			return false, nil
		}
		return time.Since(msg.Date) < time.Duration(days)*24*time.Hour, nil
	case "before":
		target, err := time.Parse("2006-01-02", c.Value)
		if err != nil {
			return false, err
		}
		if msg.Date.IsZero() {
			return false, nil
		}
		return msg.Date.Before(target), nil
	case "after":
		target, err := time.Parse("2006-01-02", c.Value)
		if err != nil {
			return false, err
		}
		if msg.Date.IsZero() {
			return false, nil
		}
		return msg.Date.After(target), nil
	}

	val := getFieldValueWithExtras(c.Field, msg, extras)

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
		ok = c.CompiledRegex != nil && c.CompiledRegex.MatchString(val)
	}
	slog.Debug("condition evaluated", "field", c.Field, "op", c.Operator, "expected", c.Value, "actual", val, "match", ok)
	return ok, nil
}

func parseDays(v string) (int, error) {
	var days int
	n, _ := fmt.Sscanf(v, "%d days", &days)
	if n != 1 {
		return 0, fmt.Errorf("invalid days format: %s (expected 'N days')", v)
	}
	return days, nil
}

func getFieldValueWithExtras(field string, msg *imap.Message, extras *msgExtras) string {
	if strings.HasPrefix(field, "header:") {
		name := strings.TrimPrefix(field, "header:")
		if extras != nil {
			if v, ok := extras.headers[name]; ok {
				return v
			}
		}
		return ""
	}
	switch field {
	case "from":
		return addrsToString(msg.From)
	case "to":
		return addrsToString(msg.To)
	case "cc":
		return addrsToString(msg.Cc)
	case "subject":
		return decodeMIME(msg.Subject)
	case "body":
		if extras != nil {
			return extras.body
		}
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
