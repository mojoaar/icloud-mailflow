package rules

import (
	"testing"

	"github.com/mojoaar/icloud-mailflow/internal/db"
	"github.com/mojoaar/icloud-mailflow/internal/imap"
)

func makeMsg(subject, fromEmail string) *imap.Message {
	return &imap.Message{
		UID:     1,
		Subject: subject,
		From:    []imap.Address{{Name: "", Email: fromEmail}},
	}
}

func makeMsgFull(subject string, from, to, cc []imap.Address) *imap.Message {
	return &imap.Message{
		UID:     1,
		Subject: subject,
		From:    from,
		To:      to,
		Cc:      cc,
	}
}

func makeRule(name string, enabled bool, priority int, groups []db.ConditionGroup) db.Rule {
	return db.Rule{
		ID:       1,
		Name:     name,
		Enabled:  enabled,
		Priority: priority,
		Groups:   groups,
	}
}

func makeGroup(op string, conditions []db.Condition) db.ConditionGroup {
	return db.ConditionGroup{
		Operator:   op,
		Conditions: conditions,
	}
}

func makeCond(field, operator, value string) db.Condition {
	return db.Condition{Field: field, Operator: operator, Value: value}
}

func TestMatchFirstMatchingRule(t *testing.T) {
	rules := []db.Rule{
		makeRule("rule1", true, 0, []db.ConditionGroup{
			makeGroup("AND", []db.Condition{makeCond("from", "equals", "alice@example.com")}),
		}),
		makeRule("rule2", true, 1, []db.ConditionGroup{
			makeGroup("AND", []db.Condition{makeCond("from", "equals", "bob@example.com")}),
		}),
	}

	msg := makeMsg("test", "bob@example.com")
	matched, err := Match(rules, msg)
	if err != nil {
		t.Fatalf("Match: %v", err)
	}
	if matched == nil {
		t.Fatal("expected a match")
	}
	if matched.Name != "rule2" {
		t.Errorf("matched rule = %q, want rule2", matched.Name)
	}
}

func TestMatchReturnsFirstMatchingOnly(t *testing.T) {
	rules := []db.Rule{
		makeRule("rule1", true, 0, []db.ConditionGroup{
			makeGroup("AND", []db.Condition{makeCond("subject", "contains", "hello")}),
		}),
		makeRule("rule2", true, 1, []db.ConditionGroup{
			makeGroup("AND", []db.Condition{makeCond("subject", "contains", "hello")}),
		}),
	}

	msg := makeMsg("hello world", "a@b.com")
	matched, err := Match(rules, msg)
	if err != nil {
		t.Fatalf("Match: %v", err)
	}
	if matched.Name != "rule1" {
		t.Errorf("should match first rule, got %q", matched.Name)
	}
}

func TestMatchSkipsDisabledRules(t *testing.T) {
	rules := []db.Rule{
		makeRule("rule1", false, 0, []db.ConditionGroup{
			makeGroup("AND", []db.Condition{makeCond("from", "equals", "a@b.com")}),
		}),
		makeRule("rule2", true, 1, []db.ConditionGroup{
			makeGroup("AND", []db.Condition{makeCond("from", "equals", "a@b.com")}),
		}),
	}

	msg := makeMsg("test", "a@b.com")
	matched, err := Match(rules, msg)
	if err != nil {
		t.Fatalf("Match: %v", err)
	}
	if matched.Name != "rule2" {
		t.Errorf("expected rule2, got %q", matched.Name)
	}
}

func TestMatchNoMatch(t *testing.T) {
	rules := []db.Rule{
		makeRule("rule1", true, 0, []db.ConditionGroup{
			makeGroup("AND", []db.Condition{makeCond("from", "equals", "x@y.com")}),
		}),
	}

	msg := makeMsg("test", "a@b.com")
	matched, err := Match(rules, msg)
	if err != nil {
		t.Fatalf("Match: %v", err)
	}
	if matched != nil {
		t.Errorf("expected nil, got %q", matched.Name)
	}
}

func TestMatchEmptyRules(t *testing.T) {
	matched, err := Match([]db.Rule{}, makeMsg("test", "a@b.com"))
	if err != nil {
		t.Fatalf("Match: %v", err)
	}
	if matched != nil {
		t.Error("expected nil match for empty rules")
	}
}

func TestEvaluateEmptyGroups(t *testing.T) {
	rule := makeRule("test", true, 0, []db.ConditionGroup{})
	ok, err := Evaluate(&rule, makeMsg("test", "a@b.com"))
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !ok {
		t.Error("Evaluate with empty groups should return true")
	}
}

func TestEvaluateConditionEquals(t *testing.T) {
	rule := makeRule("test", true, 0, []db.ConditionGroup{
		makeGroup("AND", []db.Condition{makeCond("from", "equals", "alice@example.com")}),
	})
	ok, err := Evaluate(&rule, makeMsg("test", "alice@example.com"))
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !ok {
		t.Error("equals should match")
	}
}

func TestEvaluateConditionEqualsCaseInsensitive(t *testing.T) {
	rule := makeRule("test", true, 0, []db.ConditionGroup{
		makeGroup("AND", []db.Condition{makeCond("from", "equals", "Alice@Example.com")}),
	})
	ok, err := Evaluate(&rule, makeMsg("test", "alice@example.com"))
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !ok {
		t.Error("equals should be case-insensitive")
	}
}

func TestEvaluateConditionNotEquals(t *testing.T) {
	rule := makeRule("test", true, 0, []db.ConditionGroup{
		makeGroup("AND", []db.Condition{makeCond("from", "not_equals", "spam@example.com")}),
	})
	ok, err := Evaluate(&rule, makeMsg("test", "alice@example.com"))
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !ok {
		t.Error("not_equals should match")
	}

	ok, err = Evaluate(&rule, makeMsg("test", "spam@example.com"))
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if ok {
		t.Error("not_equals should not match same value")
	}
}

func TestEvaluateConditionContains(t *testing.T) {
	rule := makeRule("test", true, 0, []db.ConditionGroup{
		makeGroup("AND", []db.Condition{makeCond("subject", "contains", "invoice")}),
	})
	ok, err := Evaluate(&rule, makeMsg("Your Invoice #12345", "a@b.com"))
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !ok {
		t.Error("contains should match")
	}
}

func TestEvaluateConditionNotContains(t *testing.T) {
	rule := makeRule("test", true, 0, []db.ConditionGroup{
		makeGroup("AND", []db.Condition{makeCond("subject", "not_contains", "spam")}),
	})
	ok, err := Evaluate(&rule, makeMsg("Hello World", "a@b.com"))
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !ok {
		t.Error("not_contains should match")
	}

	ok, err = Evaluate(&rule, makeMsg("This is spam mail", "a@b.com"))
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if ok {
		t.Error("not_contains should not match when word present")
	}
}

func TestEvaluateConditionStartsWith(t *testing.T) {
	rule := makeRule("test", true, 0, []db.ConditionGroup{
		makeGroup("AND", []db.Condition{makeCond("subject", "starts_with", "Re:")}),
	})
	ok, err := Evaluate(&rule, makeMsg("Re: Your Email", "a@b.com"))
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !ok {
		t.Error("starts_with should match")
	}

	ok, err = Evaluate(&rule, makeMsg("Fwd: Re: Your Email", "a@b.com"))
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if ok {
		t.Error("starts_with should not match when prefix is absent")
	}
}

func TestEvaluateConditionEndsWith(t *testing.T) {
	rule := makeRule("test", true, 0, []db.ConditionGroup{
		makeGroup("AND", []db.Condition{makeCond("from", "ends_with", "@example.com")}),
	})
	ok, err := Evaluate(&rule, makeMsg("test", "alice@example.com"))
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !ok {
		t.Error("ends_with should match")
	}

	ok, err = Evaluate(&rule, makeMsg("test", "alice@other.org"))
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if ok {
		t.Error("ends_with should not match")
	}
}

func TestEvaluateConditionMatchesRegex(t *testing.T) {
	rule := makeRule("test", true, 0, []db.ConditionGroup{
		makeGroup("AND", []db.Condition{makeCond("subject", "matches_regex", `\d{3,}`)}),
	})
	ok, err := Evaluate(&rule, makeMsg("Order #12345 confirmed", "a@b.com"))
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !ok {
		t.Error("matches_regex should match")
	}

	ok, err = Evaluate(&rule, makeMsg("No numbers here", "a@b.com"))
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if ok {
		t.Error("matches_regex should not match")
	}
}

func TestEvaluateConditionInvalidRegex(t *testing.T) {
	rule := makeRule("test", true, 0, []db.ConditionGroup{
		makeGroup("AND", []db.Condition{makeCond("subject", "matches_regex", `[invalid`)}),
	})
	_, err := Evaluate(&rule, makeMsg("test", "a@b.com"))
	if err == nil {
		t.Error("invalid regex should return error")
	}
}

func TestEvaluateConditionExists(t *testing.T) {
	rule := makeRule("test", true, 0, []db.ConditionGroup{
		makeGroup("AND", []db.Condition{makeCond("from", "exists", "")}),
	})
	ok, err := Evaluate(&rule, makeMsg("test", "alice@example.com"))
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !ok {
		t.Error("exists should match when field has value")
	}
}

func TestEvaluateConditionNotExists(t *testing.T) {
	rule := makeRule("test", true, 0, []db.ConditionGroup{
		makeGroup("AND", []db.Condition{makeCond("body", "not_exists", "")}),
	})
	ok, err := Evaluate(&rule, &imap.Message{UID: 1})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !ok {
		t.Error("not_exists should match when field is empty")
	}
}

func TestEvaluateHasAttachmentExists(t *testing.T) {
	rule := makeRule("test", true, 0, []db.ConditionGroup{
		makeGroup("AND", []db.Condition{makeCond("has_attachment", "exists", "")}),
	})
	ok, err := Evaluate(&rule, &imap.Message{UID: 1, HasAttach: true})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !ok {
		t.Error("has_attachment exists should match when true")
	}
}

func TestEvaluateHasAttachmentNotExists(t *testing.T) {
	rule := makeRule("test", true, 0, []db.ConditionGroup{
		makeGroup("AND", []db.Condition{makeCond("has_attachment", "not_exists", "")}),
	})
	ok, err := Evaluate(&rule, &imap.Message{UID: 1, HasAttach: false})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !ok {
		t.Error("has_attachment not_exists should match when false")
	}
}

func TestEvaluateANDLogicAllTrue(t *testing.T) {
	rule := makeRule("test", true, 0, []db.ConditionGroup{
		makeGroup("AND", []db.Condition{
			makeCond("from", "equals", "alice@example.com"),
			makeCond("subject", "contains", "hello"),
		}),
	})
	ok, err := Evaluate(&rule, makeMsg("hello world", "alice@example.com"))
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !ok {
		t.Error("AND should match when all conditions are true")
	}
}

func TestEvaluateANDLogicOneFalse(t *testing.T) {
	rule := makeRule("test", true, 0, []db.ConditionGroup{
		makeGroup("AND", []db.Condition{
			makeCond("from", "equals", "alice@example.com"),
			makeCond("subject", "contains", "goodbye"),
		}),
	})
	ok, err := Evaluate(&rule, makeMsg("hello world", "alice@example.com"))
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if ok {
		t.Error("AND should not match when one condition is false")
	}
}

func TestEvaluateORLogicOneTrue(t *testing.T) {
	rule := makeRule("test", true, 0, []db.ConditionGroup{
		makeGroup("OR", []db.Condition{
			makeCond("from", "equals", "alice@example.com"),
			makeCond("from", "equals", "bob@example.com"),
		}),
	})
	ok, err := Evaluate(&rule, makeMsg("test", "alice@example.com"))
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !ok {
		t.Error("OR should match when one condition is true")
	}
}

func TestEvaluateORLogicAllFalse(t *testing.T) {
	rule := makeRule("test", true, 0, []db.ConditionGroup{
		makeGroup("OR", []db.Condition{
			makeCond("from", "equals", "alice@example.com"),
			makeCond("from", "equals", "bob@example.com"),
		}),
	})
	ok, err := Evaluate(&rule, makeMsg("test", "carol@example.com"))
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if ok {
		t.Error("OR should not match when all conditions are false")
	}
}

func TestEvaluateMultipleGroupsAND(t *testing.T) {
	rule := makeRule("test", true, 0, []db.ConditionGroup{
		makeGroup("AND", []db.Condition{makeCond("from", "equals", "alice@example.com")}),
		makeGroup("AND", []db.Condition{makeCond("subject", "contains", "hello")}),
	})
	ok, err := Evaluate(&rule, makeMsg("hello world", "alice@example.com"))
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !ok {
		t.Error("multiple AND groups should all be true")
	}
}

func TestEvaluateMultipleGroupsOneFails(t *testing.T) {
	rule := makeRule("test", true, 0, []db.ConditionGroup{
		makeGroup("AND", []db.Condition{makeCond("from", "equals", "alice@example.com")}),
		makeGroup("AND", []db.Condition{makeCond("subject", "contains", "goodbye")}),
	})
	ok, err := Evaluate(&rule, makeMsg("hello world", "alice@example.com"))
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if ok {
		t.Error("should be false when one group fails")
	}
}

func TestGetFieldValueFrom(t *testing.T) {
	msg := makeMsgFull("test", []imap.Address{{Name: "Alice", Email: "alice@example.com"}}, nil, nil)
	v := getFieldValue("from", msg)
	if v != "alice@example.com" {
		t.Errorf("got %q, want alice@example.com", v)
	}
}

func TestGetFieldValueTo(t *testing.T) {
	msg := makeMsgFull("test", nil, []imap.Address{{Email: "bob@example.com"}}, nil)
	v := getFieldValue("to", msg)
	if v != "bob@example.com" {
		t.Errorf("got %q, want bob@example.com", v)
	}
}

func TestGetFieldValueCc(t *testing.T) {
	msg := makeMsgFull("test", nil, nil, []imap.Address{{Email: "carol@example.com"}})
	v := getFieldValue("cc", msg)
	if v != "carol@example.com" {
		t.Errorf("got %q, want carol@example.com", v)
	}
}

func TestGetFieldValueSubject(t *testing.T) {
	msg := makeMsg("Hello World", "a@b.com")
	v := getFieldValue("subject", msg)
	if v != "Hello World" {
		t.Errorf("got %q, want Hello World", v)
	}
}

func TestGetFieldValueBody(t *testing.T) {
	msg := &imap.Message{UID: 1}
	v := getFieldValue("body", msg)
	if v != "" {
		t.Errorf("body should return empty string, got %q", v)
	}
}

func TestGetFieldValueHeaderPrefix(t *testing.T) {
	msg := &imap.Message{UID: 1}
	v := getFieldValue("header:X-Custom", msg)
	if v != "" {
		t.Errorf("header field should return empty string, got %q", v)
	}
}

func TestGetFieldValueUnknownField(t *testing.T) {
	msg := &imap.Message{UID: 1}
	v := getFieldValue("unknown_field", msg)
	if v != "" {
		t.Errorf("unknown field should return empty string, got %q", v)
	}
}

func TestAddrsToStringMultipleAddresses(t *testing.T) {
	addrs := []imap.Address{
		{Name: "Alice", Email: "alice@example.com"},
		{Email: "bob@example.com"},
	}
	v := addrsToString(addrs)
	expected := "alice@example.com, bob@example.com"
	if v != expected {
		t.Errorf("got %q, want %q", v, expected)
	}
}

func TestAddrsToStringEmpty(t *testing.T) {
	v := addrsToString([]imap.Address{})
	if v != "" {
		t.Errorf("expected empty string, got %q", v)
	}
}

func TestDecodeMIMESimpleString(t *testing.T) {
	v := decodeMIME("Hello")
	if v != "Hello" {
		t.Errorf("got %q, want Hello", v)
	}
}

func TestEvaluateEmptyConditionsInGroupAND(t *testing.T) {
	rule := makeRule("test", true, 0, []db.ConditionGroup{
		{Operator: "AND"},
	})
	ok, err := Evaluate(&rule, makeMsg("test", "a@b.com"))
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !ok {
		t.Error("empty AND group should return true")
	}
}

func TestEvaluateEmptyConditionsInGroupOR(t *testing.T) {
	rule := makeRule("test", true, 0, []db.ConditionGroup{
		{Operator: "OR"},
	})
	ok, err := Evaluate(&rule, makeMsg("test", "a@b.com"))
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if ok {
		t.Error("empty OR group should return false")
	}
}
