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

func Match(rules []db.Rule, msg *imap.Message, client imap.Client, loc *time.Location) (*db.Rule, map[string]string, error) {
	for i := range rules {
		if !rules[i].Enabled {
			continue
		}
		ok, captures, err := Evaluate(&rules[i], msg, client)
		if err != nil {
			return nil, nil, fmt.Errorf("rule %d (%s): %w", rules[i].ID, rules[i].Name, err)
		}
		if ok {
			if !inSchedule(&rules[i], loc) {
				continue
			}
			return &rules[i], captures, nil
		}
	}
	return nil, nil, nil
}

func inSchedule(rule *db.Rule, loc *time.Location) bool {
	if rule.ScheduleDays == "" && rule.ScheduleStart == "" && rule.ScheduleEnd == "" {
		return true
	}
	now := time.Now().In(loc)
	day := strings.ToLower(now.Format("mon"))
	days := strings.ToLower(rule.ScheduleDays)
	if days != "" && !strings.Contains(days, day) {
		return false
	}
	current := now.Format("15:04")
	if rule.ScheduleStart != "" && current < rule.ScheduleStart {
		return false
	}
	if rule.ScheduleEnd != "" && current > rule.ScheduleEnd {
		return false
	}
	return true
}

func Evaluate(rule *db.Rule, msg *imap.Message, client imap.Client) (bool, map[string]string, error) {
	if len(rule.Groups) == 0 {
		return true, nil, nil
	}
	captures := make(map[string]string)
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
		ok, err := evalGroupWithExtras(group, msg, extras, captures)
		if err != nil {
			return false, nil, err
		}
		if !ok {
			return false, nil, nil
		}
	}
	return len(rule.Groups) > 0, captures, nil
}

func evalGroupWithExtras(g db.ConditionGroup, msg *imap.Message, extras *msgExtras, captures map[string]string) (bool, error) {
	for _, c := range g.Conditions {
		ok, err := evalConditionWithExtras(c, msg, extras, captures)
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
		ok, err := evalGroupWithExtras(sub, msg, extras, captures)
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

func evalConditionWithExtras(c db.Condition, msg *imap.Message, extras *msgExtras, captures map[string]string) (bool, error) {
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
		if c.CompiledRegex == nil {
			return false, nil
		}
		matches := c.CompiledRegex.FindStringSubmatch(val)
		if matches == nil {
			return false, nil
		}
		for i, name := range c.CompiledRegex.SubexpNames() {
			if i == 0 {
				captures["capture:0"] = matches[0]
			} else if name != "" && i < len(matches) {
				captures["capture:"+name] = matches[i]
			}
		}
		ok = true
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
		return AddrsToString(msg.From)
	case "to":
		return AddrsToString(msg.To)
	case "cc":
		return AddrsToString(msg.Cc)
	case "subject":
		return decodeMIME(msg.Subject)
	case "content_type":
		return strings.Join(msg.ContentTypes, ", ")
	case "body":
		if extras != nil {
			return extras.body
		}
		return ""
	}
	return ""
}

func AddrsToString(addrs []imap.Address) string {
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

type ConditionResult struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Expected string `json:"expected"`
	Passed   bool   `json:"passed"`
	Actual   string `json:"actual"`
}

type GroupResult struct {
	Operator string            `json:"operator"`
	Passed   bool              `json:"passed"`
	Results  []ConditionResult `json:"results,omitempty"`
	Groups   []GroupResult     `json:"groups,omitempty"`
}

func EvaluateWithResults(rule *db.Rule, msg *imap.Message, client imap.Client) (bool, map[string]string, []GroupResult, error) {
	if len(rule.Groups) == 0 {
		return true, nil, nil, nil
	}
	captures := make(map[string]string)
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
	allResults := make([]GroupResult, len(rule.Groups))
	for i, group := range rule.Groups {
		gr, err := evalGroupWithResults(group, msg, extras, captures)
		if err != nil {
			return false, nil, nil, err
		}
		allResults[i] = gr
		if !gr.Passed {
			return false, captures, allResults, nil
		}
	}
	return len(rule.Groups) > 0, captures, allResults, nil
}

func evalGroupWithResults(g db.ConditionGroup, msg *imap.Message, extras *msgExtras, captures map[string]string) (GroupResult, error) {
	result := GroupResult{Operator: g.Operator}
	allPassed := true
	anyPassed := false
	for _, c := range g.Conditions {
		cr, err := evalConditionWithResult(c, msg, extras, captures)
		if err != nil {
			return result, err
		}
		result.Results = append(result.Results, cr)
		if cr.Passed {
			anyPassed = true
		} else {
			allPassed = false
		}
	}
	for _, sub := range g.Groups {
		sr, err := evalGroupWithResults(sub, msg, extras, captures)
		if err != nil {
			return result, err
		}
		result.Groups = append(result.Groups, sr)
		if sr.Passed {
			anyPassed = true
		} else {
			allPassed = false
		}
	}
	if g.Operator == "AND" {
		result.Passed = allPassed
	} else {
		result.Passed = anyPassed
	}
	return result, nil
}

func evalConditionWithResult(c db.Condition, msg *imap.Message, extras *msgExtras, captures map[string]string) (ConditionResult, error) {
	cr := ConditionResult{
		Field:    c.Field,
		Operator: c.Operator,
		Expected: c.Value,
	}

	if c.Field == "has_attachment" {
		if c.Operator == "exists" {
			if msg.HasAttach {
				cr.Actual = "true"
			} else {
				cr.Actual = "false"
			}
			cr.Passed = msg.HasAttach
			return cr, nil
		}
		if c.Operator == "not_exists" {
			if msg.HasAttach {
				cr.Actual = "true"
			} else {
				cr.Actual = "false"
			}
			cr.Passed = !msg.HasAttach
			return cr, nil
		}
	}

	switch c.Operator {
	case "older_than":
		days, err := parseDays(c.Value)
		if err != nil {
			return cr, err
		}
		if msg.Date.IsZero() {
			cr.Actual = "no date"
			return cr, nil
		}
		age := int(time.Since(msg.Date).Hours() / 24)
		cr.Actual = fmt.Sprintf("%d days", age)
		cr.Passed = time.Since(msg.Date) > time.Duration(days)*24*time.Hour
		return cr, nil
	case "newer_than":
		days, err := parseDays(c.Value)
		if err != nil {
			return cr, err
		}
		if msg.Date.IsZero() {
			cr.Actual = "no date"
			return cr, nil
		}
		age := int(time.Since(msg.Date).Hours() / 24)
		cr.Actual = fmt.Sprintf("%d days", age)
		cr.Passed = time.Since(msg.Date) < time.Duration(days)*24*time.Hour
		return cr, nil
	case "before":
		target, err := time.Parse("2006-01-02", c.Value)
		if err != nil {
			return cr, err
		}
		if msg.Date.IsZero() {
			cr.Actual = "no date"
			return cr, nil
		}
		cr.Actual = msg.Date.Format("2006-01-02")
		cr.Passed = msg.Date.Before(target)
		return cr, nil
	case "after":
		target, err := time.Parse("2006-01-02", c.Value)
		if err != nil {
			return cr, err
		}
		if msg.Date.IsZero() {
			cr.Actual = "no date"
			return cr, nil
		}
		cr.Actual = msg.Date.Format("2006-01-02")
		cr.Passed = msg.Date.After(target)
		return cr, nil
	}

	val := getFieldValueWithExtras(c.Field, msg, extras)
	cr.Actual = val

	switch c.Operator {
	case "exists":
		cr.Passed = val != ""
	case "not_exists":
		cr.Passed = val == ""
	case "equals":
		cr.Passed = strings.EqualFold(val, c.Value)
	case "not_equals":
		cr.Passed = !strings.EqualFold(val, c.Value)
	case "contains":
		cr.Passed = strings.Contains(strings.ToLower(val), strings.ToLower(c.Value))
	case "not_contains":
		cr.Passed = !strings.Contains(strings.ToLower(val), strings.ToLower(c.Value))
	case "starts_with":
		cr.Passed = strings.HasPrefix(strings.ToLower(val), strings.ToLower(c.Value))
	case "ends_with":
		cr.Passed = strings.HasSuffix(strings.ToLower(val), strings.ToLower(c.Value))
	case "matches_regex":
		if c.CompiledRegex == nil {
			return cr, nil
		}
		matches := c.CompiledRegex.FindStringSubmatch(val)
		if matches == nil {
			return cr, nil
		}
		for i, name := range c.CompiledRegex.SubexpNames() {
			if i == 0 {
				captures["capture:0"] = matches[0]
			} else if name != "" && i < len(matches) {
				captures["capture:"+name] = matches[i]
			}
		}
		cr.Passed = true
	}
	return cr, nil
}
