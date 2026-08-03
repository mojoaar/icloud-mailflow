# 7 Feature Implementation Plan — icloud-mailflow v0.8.0+

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add 7 features: CI test workflow, Prometheus /metrics, rule time/day scheduling, richer auto-reply templating + regex capture, web UI rule dry-run, webhook action type, bulk rule apply to folders.

**Architecture:** Independent features first (CI, Prometheus), then rules/poller chain (scheduling → templating → dry-run → webhook → bulk-apply). No new packages except `prometheus/client_golang`. Reuses existing patterns: handler closures over deps, HTMX partials, IMAP client interface, MCP tool registration.

**Tech Stack:** Go 1.25+, modernc.org/sqlite (no CGO), chi v5, go-imap/v2, mcp-go, prometheus/client_golang.

## Global Constraints

- `go test ./... && go vet ./...` passes after each feature
- `gofmt -l` on all touched files; do NOT reformat pre-existing unformatted files (activity.go, dashboard.go, docs.go, rules_test.go, stats.go)
- Action declared order never reordered; iCloud \Seen-before-MOVE; single-pass sequential execution
- Version stays at 0.8.0 — no bump; all changelog entries under `[Unreleased]`
- Update docs.html + README.md + AGENTS.md when features add routes, actions, packages, or MCP tools
- `SetMaxOpenConns(1)` — SQLite serializes all access
- Poller struct autoReplyRepo + sendMail fields must not regress
- Engine.Match: `func Match(rules []db.Rule, msg *imap.Message, client imap.Client) (*db.Rule, error)`
- Engine.Evaluate: `func Evaluate(rule *db.Rule, msg *imap.Message, client imap.Client) (bool, error)`
- Templates embedded via `//go:embed templates/*.html` in web/render.go
- Web handlers return `http.HandlerFunc` closure

---

### Task 1: CI Test Workflow

**Files:**
- Create: `.github/workflows/test.yml`

**Goal:** Run `go test ./... && go vet ./...` on every push and PR to main.

- [ ] **Step 1: Create workflow file**

```yaml
name: Test

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

permissions:
  contents: read

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: "^1.25.0"

      - run: go test ./...
      - run: go vet ./...
```

- [ ] **Step 2: Verify format** — `cat .github/workflows/test.yml` shows correct content
- [ ] **Step 3: Commit**

```
git add .github/workflows/test.yml
git commit -m "feat: add CI test workflow (go test + go vet) for main pushes and PRs"
```

---

### Task 2: Prometheus /metrics Endpoint

**Files:**
- Create: `internal/web/prometheus.go`
- Create: `internal/web/prometheus_test.go`
- Modify: `internal/web/web.go:97` (add route)
- Modify: `go.mod` (add `github.com/prometheus/client_golang/prometheus` and `promauto`)

**Interfaces:**
- Produces: `prometheus.go` exports package-level metrics variables (counters, gauges, histogram) and a `PromHandler() http.Handler` func
- Consumed by: Task 4 (poller.go imports prometheus package for direct Inc() calls)

- [ ] **Step 1: Add dependency**

```bash
go get github.com/prometheus/client_golang@latest
```

- [ ] **Step 2: Write prometheus.go**

```go
package web

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	MetricMessagesProcessed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "mailflow_messages_processed_total",
		Help: "Total number of messages processed by the poller.",
	})
	MetricRulesMatched = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "mailflow_rules_matched_total",
		Help: "Total number of rule matches.",
	}, []string{"rule"})
	MetricActionsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "mailflow_actions_total",
		Help: "Total number of actions executed.",
	}, []string{"type", "status"})
	MetricErrorsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "mailflow_errors_total",
		Help: "Total number of processing errors.",
	})
	MetricPollerTicks = promauto.NewCounter(prometheus.CounterOpts{
		Name: "mailflow_poller_ticks_total",
		Help: "Total number of poller ticks.",
	})
	MetricPollerTickDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "mailflow_poller_tick_duration_seconds",
		Help:    "Duration of poller ticks in seconds.",
		Buckets: prometheus.DefBuckets,
	})
	MetricCPUPercent = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "mailflow_cpu_percent",
		Help: "Current CPU usage percent (Getrusage).",
	})
	MetricMemoryBytes = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "mailflow_memory_bytes",
		Help: "Current memory usage in bytes.",
	})
	MetricUptimeSeconds = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "mailflow_uptime_seconds",
		Help: "Process uptime in seconds.",
	})
)

func PromHandler() http.Handler {
	return promhttp.Handler()
}
```

- [ ] **Step 3: Register /metrics route in web.go**

In `internal/web/web.go`, after the `/health` route (line ~97), add:

```go
r.Get("/metrics", PromHandler().ServeHTTP)
```

The route is outside auth middleware (the `r.Group` with `authMiddleware` starts at line ~101), so /metrics is public like /health.

- [ ] **Step 4: Write prometheus_test.go**

```go
package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMetricsEndpoint(t *testing.T) {
	h := PromHandler()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if len(body) == 0 {
		t.Error("metrics body should not be empty")
	}
	if !contains(body, "mailflow_") {
		t.Error("metrics body should contain mailflow_ metrics")
	}
}

func contains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
```

- [ ] **Step 5: Run tests**

```bash
go test ./internal/web/ -run TestMetrics
```

Expected: PASS.

- [ ] **Step 6: Instrument poller.go** — wait, this is done in a separate task after poller changes. For now, just the endpoint exists with zero-valued metrics.

- [ ] **Step 7: Commit**

```
git add go.mod go.sum internal/web/prometheus.go internal/web/prometheus_test.go internal/web/web.go
git commit -m "feat: add Prometheus /metrics endpoint with counters, gauges, and histogram"
```

---

### Task 3: Rule Time/Day Scheduling

**Files:**
- Modify: `internal/db/migrate.go` (add columns to `rules` table)
- Modify: `internal/db/rules.go` (add ScheduleDays/ScheduleStart/ScheduleEnd to Rule struct)
- Modify: `internal/rules/engine.go` (add `inSchedule` check to `Match`)
- Modify: `internal/web/rules.go` (parse schedule from form in `parseConditions`, pass to template)
- Modify: `internal/web/templates/rules_form.html` (add day checkboxes + time inputs)
- Modify: `internal/mcp/server.go` (accept schedule in `create_rule`/`update_rule`)
- Test: `internal/web/rules_test.go` (add TestCreateRuleWithSchedule, TestCreateRulePartialSchedule)
- Test: `internal/db/rules_test.go` (add schedule round-trip test)

**Interfaces:**
- Produces: `db.Rule` gets three new fields: `ScheduleDays string`, `ScheduleStart string`, `ScheduleEnd string` (all empty = no schedule)
- Consumes: engine.Match checks schedule after rule evaluation; web handlers parse form arrays `schedule_days[]`; mcp tools accept `schedule_days`/`schedule_start`/`schedule_end` args

**Design summary:** Schedule is optional. When all three fields are non-empty, the rule only matches if the current day (lowercase 3-letter: mon,tue,wed,thu,fri,sat,sun) is in `ScheduleDays` and current time (HH:MM) is between `ScheduleStart` and `ScheduleEnd`. Uses the app's timezone setting (from `settingsRepo`).

- [ ] **Step 1: Add migration**

In `internal/db/migrate.go`, append to `migrations` slice (before the `idx_condition_groups_rule_id` index):

```go
"ALTER TABLE rules ADD COLUMN schedule_days TEXT NOT NULL DEFAULT ''",
"ALTER TABLE rules ADD COLUMN schedule_start TEXT NOT NULL DEFAULT ''",
"ALTER TABLE rules ADD COLUMN schedule_end TEXT NOT NULL DEFAULT ''",
```

- [ ] **Step 2: Add fields to db.Rule struct**

In `internal/db/rules.go`, add after the `Enabled` field (line ~15):

```go
ScheduleDays  string `json:"schedule_days"`
ScheduleStart string `json:"schedule_start"`
ScheduleEnd   string `json:"schedule_end"`
```

- [ ] **Step 3: Update db.RulesRepo.Create** 

In `internal/db/rules.go`, in the `Create` method, update the INSERT query to include the new columns:

```go
result, err := tx.Exec(`
    INSERT INTO rules (name, description, priority, enabled, schedule_days, schedule_start, schedule_end, created_at, updated_at)
    VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
`, rule.Name, rule.Description, rule.Priority, rule.Enabled,
    rule.ScheduleDays, rule.ScheduleStart, rule.ScheduleEnd,
    now, now)
```

And in `Update`:

```go
_, err = tx.Exec(`
    UPDATE rules SET name=?, description=?, priority=?, enabled=?,
    schedule_days=?, schedule_start=?, schedule_end=?, updated_at=?
    WHERE id=?
`, rule.Name, rule.Description, rule.Priority, rule.Enabled,
    rule.ScheduleDays, rule.ScheduleStart, rule.ScheduleEnd,
    now, rule.ID)
```

And in `loadConditions` (where rules are SELECTed), add the columns to the query:

```go
rows, err := r.DB.Query(`
    SELECT id, name, description, priority, enabled, schedule_days, schedule_start, schedule_end, created_at, updated_at
    FROM rules ORDER BY priority, id
`)
```

Update the Scan to include `&rule.ScheduleDays, &rule.ScheduleStart, &rule.ScheduleEnd`.

- [ ] **Step 4: Add schedule check to engine.Match**

In `internal/rules/engine.go`, add a helper function at the bottom:

```go
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
```

In `Match`, after the `if ok` check and before returning `&rules[i]`, add:

```go
if ok {
    if !inSchedule(&rules[i], loc) {
        continue
    }
    return &rules[i], nil
}
```

Pass `loc *time.Location` into `Match`. Update `Match` signature to add `loc *time.Location` parameter:

```go
func Match(rules []db.Rule, msg *imap.Message, client imap.Client, loc *time.Location) (*db.Rule, error) {
```

Update all callers of `Match`:
- `internal/poller/poller.go` process(): pass `p.timeLocation()` (needs new method or direct load from settings)
- `internal/mcp/server.go` check_email: pass `time.UTC` (or load from settings)

Add `timeLocation()` method to Poller:

```go
func (p *Poller) timeLocation() *time.Location {
	tz, _ := p.settingsRepo.Get("timezone")
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return time.UTC
	}
	return loc
}
```

- [ ] **Step 5: Add schedule fields to web rules handlers**

In `internal/web/rules.go`, update `rulesNewHandler` data map to include:

```go
"ScheduleDays": []string{}, "ScheduleStart": "", "ScheduleEnd": "",
```

Update `rulesEditHandler` to pass schedule from the loaded rule:
```go
data["ScheduleDays"] = rule.ScheduleDays
data["ScheduleStart"] = rule.ScheduleStart
data["ScheduleEnd"] = rule.ScheduleEnd
```

Update `parseConditions` (renamed to `parseRuleForm` or keep the name but add schedule parsing) — in `rulesCreateHandler` and `rulesUpdateHandler`, after `r.ParseForm()`, add:

```go
rule.ScheduleDays = strings.Join(r.Form["schedule_days"], ",")
rule.ScheduleStart = r.FormValue("schedule_start")
rule.ScheduleEnd = r.FormValue("schedule_end")
```

- [ ] **Step 6: Add schedule fields to rules_form.html**

In `internal/web/templates/rules_form.html`, after the priority input and before the conditions card, add a schedule section:

```html
<div class="card" id="schedule">
    <h3>Schedule (optional)</h3>
    <p class="hint">Leave blank to always run. Timezone: {{.Timezone}}</p>
    <div style="display:flex;gap:4px;flex-wrap:wrap;margin-bottom:8px">
        {{$days := split .ScheduleDays ","}}
        <label><input type="checkbox" name="schedule_days" value="mon"{{if hasDay $days "mon"}} checked{{end}}> Mon</label>
        <label><input type="checkbox" name="schedule_days" value="tue"{{if hasDay $days "tue"}} checked{{end}}> Tue</label>
        <label><input type="checkbox" name="schedule_days" value="wed"{{if hasDay $days "wed"}} checked{{end}}> Wed</label>
        <label><input type="checkbox" name="schedule_days" value="thu"{{if hasDay $days "thu"}} checked{{end}}> Thu</label>
        <label><input type="checkbox" name="schedule_days" value="fri"{{if hasDay $days "fri"}} checked{{end}}> Fri</label>
        <label><input type="checkbox" name="schedule_days" value="sat"{{if hasDay $days "sat"}} checked{{end}}> Sat</label>
        <label><input type="checkbox" name="schedule_days" value="sun"{{if hasDay $days "sun"}} checked{{end}}> Sun</label>
    </div>
    <div style="display:flex;gap:8px">
        <label>Start: <input type="time" name="schedule_start" value="{{.ScheduleStart}}"></label>
        <label>End: <input type="time" name="schedule_end" value="{{.ScheduleEnd}}"></label>
    </div>
</div>
```

Add `hasDay` and `split` to template funcs in `internal/web/render.go`:

```go
"hasDay": func(days []string, day string) bool {
    for _, d := range days {
        if d == day {
            return true
        }
    }
    return false
},
"split": func(s, sep string) []string {
    if s == "" {
        return nil
    }
    return strings.Split(s, sep)
},
```

The schedule data needs to be passed as a comma-separated string from the handler (stored in Rule), so `split` converts it to a slice for `hasDay`.

- [ ] **Step 7: Add schedule to MCP create_rule/update_rule**

In `internal/mcp/server.go`, in the `create_rule` tool, add three new string parameters:

```go
mcp.WithString("schedule_days", mcp.Description("Comma-separated days: mon,tue,wed,thu,fri,sat,sun (empty = always)")),
mcp.WithString("schedule_start", mcp.Description("Start time HH:MM (empty = no start bound)")),
mcp.WithString("schedule_end", mcp.Description("End time HH:MM (empty = no end bound)")),
```

In the handler, extract:

```go
scheduleDays, _ := args["schedule_days"].(string)
scheduleStart, _ := args["schedule_start"].(string)
scheduleEnd, _ := args["schedule_end"].(string)
```

Same for `update_rule`.

- [ ] **Step 8: Write tests**

Test rule schedule in `internal/web/rules_test.go` — add `TestCreateRuleWithSchedule` and `TestCreateRulePartialSchedule` (POST /rules with schedule fields, verify saved correctly).

Test round-trip in `internal/db/rules_test.go` — existing tests already exercise Create/Get/Update for rules; they should naturally include the new columns since the Scan was updated.

- [ ] **Step 9: Verify**

```bash
go test ./... && go vet ./...
```

Expected: all 12 packages pass.

- [ ] **Step 10: Commit**

```
git add -A
git commit -m "feat: add rule time/day scheduling (schedule_days, schedule_start, schedule_end)"
```

---

### Task 4: Richer Auto-Reply Templating + Regex Capture

**Files:**
- Modify: `internal/rules/engine.go` (change Evaluate to return captures, extract named groups)
- Modify: `internal/poller/poller.go` (receive captures from Evaluate/Match, expand `[capture:name]` and new variables in auto_reply)
- Modify: `internal/mcp/server.go` (update check_email for new Evaluate return)
- Modify: `internal/poller/poller_test.go` (update test calls to Match/Evaluate)
- Test: `internal/rules/engine_test.go` (add regex capture test)
- Test: `internal/poller/poller_test.go` (add TestAutoReplyTemplateVariables)

**Interfaces:**
- Produces: `Evaluate(rule, msg, client)` → `(bool, map[string]string, error)`; `Match(rules, msg, client, loc)` → `(*db.Rule, map[string]string, error)`
- Captures map: keyed by `capture:<name>` for named regex groups, also `capture:0` for full match
- New template vars: `[to]`, `[cc]`, `[rule_name]`, `[capture:name]`

- [ ] **Step 1: Update Evaluate to return captures**

In `internal/rules/engine.go`, change the signature:

```go
func Evaluate(rule *db.Rule, msg *imap.Message, client imap.Client) (bool, map[string]string, error) {
```

At the top of Evaluate, initialize:
```go
captures := make(map[string]string)
```

In `evalConditionWithExtras`, add a `captures map[string]string` parameter. When `c.Operator == "matches_regex"` and `c.CompiledRegex != nil`:

```go
if c.Operator == "matches_regex" {
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
    return true, nil
}
```

Update `evalConditionWithExtras` signature to `func evalConditionWithExtras(c db.Condition, msg *imap.Message, extras *msgExtras, captures map[string]string) (bool, error)`.

Update `evalGroupWithExtras` signature to pass `captures` through recursively.

Update `Evaluate` to pass `captures` into `evalGroupWithExtras`, and return `captures` at the end:

```go
return len(rule.Groups) > 0, captures, nil
```

For the "no conditions" fast path:
```go
if len(rule.Groups) == 0 {
    return true, nil, nil
}
```

For group evaluation failure:
```go
if !ok {
    return false, nil, err
}
```

- [ ] **Step 2: Update Match to thread captures**

```go
func Match(rules []db.Rule, msg *imap.Message, client imap.Client, loc *time.Location) (*db.Rule, map[string]string, error) {
```

Inside the loop, after `Evaluate`:
```go
ok, captures, err := Evaluate(&rules[i], msg, client)
if err != nil {
    return nil, nil, fmt.Errorf(...)
}
if ok {
    if !inSchedule(&rules[i], loc) {
        continue
    }
    return &rules[i], captures, nil
}
```

No match returns `nil, nil, nil`.

- [ ] **Step 3: Update p.Match caller in poller.go process()**

In `internal/poller/poller.go`, find the `rules.Match(ruleList, msg, p.imapClient)` call (~line 166). Change to:

```go
matched, captures, err := rules.Match(ruleList, msg, p.imapClient, p.timeLocation())
```

And update `p.executeActions` call to pass captures:

```go
p.executeActions(matched, uint32(uid), msg, captures)
```

Update `executeActions` signature:

```go
func (p *Poller) executeActions(rule *db.Rule, uid uint32, msg *imap.Message, captures map[string]string) {
```

- [ ] **Step 4: Extend auto_reply templating**

In the auto_reply case, after the existing `[subject]`, `[from]`, `[date]` replacements, add:

```go
body = strings.ReplaceAll(body, "[to]", addrsToString(msg.To))
body = strings.ReplaceAll(body, "[cc]", addrsToString(msg.Cc))
body = strings.ReplaceAll(body, "[rule_name]", rule.Name)

for k, v := range captures {
    body = strings.ReplaceAll(body, "["+k+"]", v)
}
```

The `addrsToString` function is in `internal/rules/engine.go` and is not exported. Either export it (`AddrsToString`) or duplicate the logic inline in poller.

Add `addrsToString` as an exported function in engine.go:

```go
func AddrsToString(addrs []imap.Address) string {
    parts := make([]string, len(addrs))
    for i, a := range addrs {
        if a.Name != "" {
            parts[i] = a.Name + " <" + a.Email + ">"
        } else {
            parts[i] = a.Email
        }
    }
    return strings.Join(parts, ", ")
}
```

Update internal uses of `addrsToString` to call `AddrsToString`.

- [ ] **Step 5: Update MCP check_email**

In `internal/mcp/server.go`, update the `rules.Match` call:

```go
matched, _, err := rules.Match(ruleList, msg, nil, time.UTC)
```

- [ ] **Step 6: Update all test callers of Match/Evaluate**

In `internal/poller/poller_test.go`, update all calls to `rules.Match` to use the new signature (add `time.UTC` or a location, and capture the second return):

```go
matched, _, err := rules.Match(ruleList, msg, mockClient, time.UTC)
```

In `internal/rules/engine_test.go`, update all `Evaluate` calls:
```go
ok, captures, err := rules.Evaluate(rule, msg, client)
```

Add a test for regex captures:
```go
func TestEvaluateRegexCaptures(t *testing.T) {
    rule := &db.Rule{
        Groups: []db.ConditionGroup{{
            Operator: "AND",
            Conditions: []db.Condition{{
                Field: "from", Operator: "matches_regex", Value: `(?P<domain>@\w+\.\w+)`,
            }},
        }},
    }
    // CompiledRegex populated by repo, or set manually for test:
    rule.Groups[0].Conditions[0].CompiledRegex = regexp.MustCompile(`(?P<domain>@\w+\.\w+)`)
    msg := &imap.Message{From: []imap.Address{{Email: "user@example.com"}}}
    ok, captures, err := Evaluate(rule, msg, nil)
    if !ok { t.Fatal("should match") }
    if captures["capture:domain"] != "@example.com" {
        t.Errorf("expected @example.com, got %q", captures["capture:domain"])
    }
}
```

- [ ] **Step 7: Test auto_reply template expansion**

In `internal/poller/poller_test.go`, add `TestAutoReplyTemplateVariables`:

```go
func TestAutoReplyTemplateVariables(t *testing.T) {
    database := db.NewTestDB(t)
    // Build Poller with autoReplyRepo + sendMail recorder
    // Create rule with auto_reply action using [to], [cc], [rule_name], [capture:name]
    // Create msg with To, Cc addresses and matching regex
    // Call executeActions with captures map
    // Assert sent email body contains expanded values
}
```

- [ ] **Step 8: Verify**

```bash
go test ./... && go vet ./...
```

Expected: all 12 packages pass. Fix any compilation errors from type signature changes.

- [ ] **Step 9: Commit**

```
git add -A
git commit -m "feat: richer auto-reply templating — regex capture groups, [to], [cc], [rule_name]"
```

---

### Task 5: Web UI Rule Dry-Run

**Files:**
- Create: none (added to existing files)
- Modify: `internal/web/rules.go` (add `rulesTestHandler` + `rulesTestMessageHandler`)
- Modify: `internal/web/web.go` (add route `POST /rules/{id}/test` + `POST /rules/{id}/test-message`)
- Modify: `internal/web/templates/rules_form.html` (add test panel)
- Modify: `internal/rules/engine.go` (add `evalGroupWithExtrasResults` for per-condition breakdown)
- Test: `internal/web/rules_test.go` (add TestRuleTestSynthetic, TestRuleTestRealMessage)

**Design summary:** Two handlers. `POST /rules/{id}/test` takes form fields (from/to/cc/subject), calls Evaluate, returns HTML partial showing match result + per-condition breakdown. `POST /rules/{id}/test-message` takes folder + message-index, fetches real message via IMAP, does full evaluate with body/header, returns same breakdown.

- [ ] **Step 1: Add per-condition result tracking to engine**

In `internal/rules/engine.go`, add a new type:

```go
type ConditionResult struct {
    Field    string `json:"field"`
    Operator string `json:"operator"`
    Value    string `json:"value"`
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
```

Add a new function that mirrors `Evaluate` but collects results:

```go
func EvaluateWithResults(rule *db.Rule, msg *imap.Message, client imap.Client) (bool, map[string]string, []GroupResult, error)
```

This is a copy of `Evaluate` with extra result collection. In `evalConditionWithExtras`, after evaluating each condition, append to a `results` slice.

Export `evalGroupWithExtras` for reuse or create a parallel version. The key change: in the condition-evaluation loop, record each condition's pass/fail + actual vs expected values.

- [ ] **Step 2: Add rulesTestHandler**

In `internal/web/rules.go`, add:

```go
func rulesTestHandler(repo *db.RulesRepo, imapClient imap.Client) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
        rule, err := repo.Get(id)
        if err != nil {
            http.Error(w, "Rule not found", http.StatusNotFound)
            return
        }
        r.ParseForm()
        msg := &imap.Message{
            Subject: r.FormValue("subject"),
            From:    []imap.Address{{Email: r.FormValue("from")}},
            To:      []imap.Address{{Email: r.FormValue("to")}},
            Cc:      []imap.Address{{Email: r.FormValue("cc")}},
        }
        matched, captures, results, err := rules.EvaluateWithResults(rule, msg, nil)
        if err != nil {
            renderPartial(w, "toast", map[string]any{"Error": err.Error()})
            return
        }
        renderPartial(w, "rules_test_result", map[string]any{
            "Matched":  matched,
            "Captures": captures,
            "Results":  results,
            "Rule":     rule,
        })
    }
}
```

- [ ] **Step 3: Add rulesTestMessageHandler**

```go
func rulesTestMessageHandler(repo *db.RulesRepo, imapClient imap.Client) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
        rule, err := repo.Get(id)
        if err != nil {
            http.Error(w, "Rule not found", http.StatusNotFound)
            return
        }
        r.ParseForm()
        folder := r.FormValue("folder")
        // Search for one message in the folder
        uids, err := imapClient.SearchMessages(folder, 1, 0)
        if err != nil || len(uids) == 0 {
            renderPartial(w, "toast", map[string]any{"Error": "No messages found in folder"})
            return
        }
        msg, err := imapClient.FetchMessage(uint32(uids[0]))
        if err != nil {
            renderPartial(w, "toast", map[string]any{"Error": "Failed to fetch message: " + err.Error()})
            return
        }
        matched, captures, results, err := rules.EvaluateWithResults(rule, msg, imapClient)
        if err != nil {
            renderPartial(w, "toast", map[string]any{"Error": err.Error()})
            return
        }
        renderPartial(w, "rules_test_result", map[string]any{
            "Matched":  matched,
            "Captures": captures,
            "Results":  results,
            "Rule":     rule,
            "Message":  msg,
        })
    }
}
```

- [ ] **Step 4: Register routes**

In `internal/web/web.go`, add routes after the existing rules routes:

```go
r.Post("/rules/{id}/test", rulesTestHandler(rulesRepo, imapClient))
r.Post("/rules/{id}/test-message", rulesTestMessageHandler(rulesRepo, imapClient))
```

Update the `rulesRepo` and `imapClient` closures — they need to be in scope. The router builder in `New()` already has them.

- [ ] **Step 5: Add template partial for test results**

Create the logic inline in rules_form.html using an existing template partial approach. The test panel HTML goes in rules_form.html after the actions card:

```html
<div class="card" id="rule-test">
    <h3>Test Rule</h3>
    <form hx-post="/rules/{{.Rule.ID}}/test" hx-target="#test-result" hx-swap="innerHTML">
        <div style="display:flex;gap:8px;flex-wrap:wrap">
            <input name="from" placeholder="From email" style="flex:1;min-width:200px">
            <input name="to" placeholder="To email" style="flex:1;min-width:200px">
            <input name="cc" placeholder="CC email" style="flex:1;min-width:200px">
            <input name="subject" placeholder="Subject" style="flex:1;min-width:200px">
        </div>
        <button type="submit">Test (Synthetic)</button>
    </form>
    <form hx-post="/rules/{{.Rule.ID}}/test-message" hx-target="#test-result" hx-swap="innerHTML" style="margin-top:8px">
        <input name="folder" placeholder="Folder name" list="folder-list" style="width:200px">
        <button type="submit">Test (Real Message)</button>
    </form>
    <div id="test-result"></div>
</div>
```

Only show when `.Rule.ID` is set (edit mode, not new rule).

- [ ] **Step 6: Write test for dry-run**

In `internal/web/rules_test.go`:

```go
func TestRuleTestSynthetic(t *testing.T) {
    database := openWebTestDB(t)
    rulesRepo := db.NewRulesRepo(database)
    // Create a rule
    rule := &db.Rule{Name: "test-rule", Enabled: true, Priority: 10}
    rule.Groups = []db.ConditionGroup{{
        Operator: "AND",
        Conditions: []db.Condition{{Field: "from", Operator: "contains", Value: "@test.com"}},
    }}
    rulesRepo.Create(rule)
    // Test it
    form := url.Values{"from": {"user@test.com"}, "subject": {"hello"}}
    req := httptest.NewRequest(http.MethodPost, "/rules/"+strconv.FormatInt(rule.ID, 10)+"/test", strings.NewReader(form.Encode()))
    req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
    rec := serveHandler(rulesTestHandler(rulesRepo, nil), req)
    if rec.Code != http.StatusOK {
        t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
    }
    // Body should indicate match
    if !strings.Contains(rec.Body.String(), "Matched") {
        t.Error("response should contain 'Matched'")
    }
}
```

- [ ] **Step 7: Verify**

```bash
go test ./... && go vet ./...
```

Expected: all 12 packages pass.

- [ ] **Step 8: Commit**

```
git add -A
git commit -m "feat: web UI rule dry-run with synthetic and real message testing"
```

---

### Task 6: Webhook Action Type

**Files:**
- Modify: `internal/poller/poller.go` (add `case "webhook":` to executeActions)
- Modify: `internal/web/templates/rules_form.html` (add webhook option to action select)
- Modify: `internal/web/templates/docs.html` (add webhook to Actions table)
- Modify: `internal/db/settings.go` or `internal/config/config.go` (add `webhook_secret` setting)
- Test: `internal/poller/poller_test.go` (add TestExecuteActionsWebhook)
- No new files needed; webhook is a new action type in the existing switch

**Interfaces:**
- Consumes: `p.sendMail` seam pattern — add `sendWebhook` seam for testability
- Consumes: settingsRepo.Get("webhook_secret") for optional X-Webhook-Secret header

- [ ] **Step 1: Add sendWebhook seam to Poller struct**

In `internal/poller/poller.go`, add a field after `sendMail`:

```go
sendWebhook func(url string, payload []byte, secret string) error
```

In `executeActions`, use:

```go
send := p.sendWebhook
if send == nil {
    send = defaultSendWebhook
}
```

Define `defaultSendWebhook` at package level in poller.go:

```go
func defaultSendWebhook(url string, payload []byte, secret string) error {
    req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
    if err != nil {
        return err
    }
    req.Header.Set("Content-Type", "application/json")
    if secret != "" {
        req.Header.Set("X-Webhook-Secret", secret)
    }
    client := &http.Client{Timeout: 10 * time.Second}
    resp, err := client.Do(req)
    if err != nil {
        return err
    }
    resp.Body.Close()
    if resp.StatusCode >= 400 {
        return fmt.Errorf("webhook returned status %d", resp.StatusCode)
    }
    return nil
}
```

Add imports to poller.go: `"bytes"`, `"encoding/json"`, `"fmt"`, `"net/http"` (some already present).

- [ ] **Step 2: Add `case "webhook":` to executeActions**

In the executeActions switch, add after the `auto_reply` case:

```go
case "webhook":
    if action.Value == "" {
        continue
    }
    payload := map[string]any{
        "rule":    rule.Name,
        "subject": subject,
        "from":    from,
        "to":      addrsToString(msg.To),
        "cc":      addrsToString(msg.Cc),
        "date":    msg.Date.Format(time.RFC3339),
        "uid":     effectiveUID,
    }
    body, _ := json.Marshal(payload)
    secret, _ := p.settingsRepo.Get("webhook_secret")
    send := p.sendWebhook
    if send == nil {
        send = defaultSendWebhook
    }
    if err := send(action.Value, body, secret); err != nil {
        slog.Error("webhook failed", "url", action.Value, "error", err)
        logAction(effectiveUID, action, "error")
    } else {
        logAction(effectiveUID, action, "success")
    }
```

- [ ] **Step 3: Add webhook to rules_form.html action select**

In the `<select name="action_type">` dropdown, add an option:

```html
<option value="webhook">Post Webhook</option>
```

Add the webhook input rendering in the action value section:

```html
{{else if eq .Type "webhook"}}
<input type="text" name="action_value" placeholder="Webhook URL (e.g. https://ntfy.sh/topic)" style="flex:1">
```

- [ ] **Step 4: Add webhook_secret to settings**

In `internal/db/settings.go`, no changes needed — the `SettingsRepo.Set` already handles arbitrary key-value pairs. The web form for settings is in `internal/web/settings.go` and `templates/settings.html`. Add a webhook secret field to settings.html (optional, not required for this task since `settingsRepo.Set("webhook_secret", value)` works via MCP or the existing settings form extensibility).

Actually, add it to the settings page for convenience. In `internal/web/templates/settings.html`, in the settings form, add:

```html
<div class="form-row">
    <label>Webhook Secret</label>
    <input type="text" name="webhook_secret" value="{{.WebhookSecret}}" placeholder="Optional X-Webhook-Secret header value">
</div>
```

In `internal/web/settings.go` `settingsSaveIMAP` or a dedicated handler (reuse the existing settings pattern):

```go
// In settingsPage, add WebhookSecret to data map
webhookSecret, _ := settingsRepo.Get("webhook_secret")
data["WebhookSecret"] = webhookSecret
```

- [ ] **Step 5: Test webhook action**

In `internal/poller/poller_test.go`:

```go
func TestExecuteActionsWebhook(t *testing.T) {
    database := db.NewTestDB(t)
    var calledURL string
    var calledBody []byte
    p := &Poller{
        imapClient: &trackedMock{},
        logRepo:    db.NewLogRepo(database),
        statsRepo:  db.NewStatsRepo(database),
        cfg:        &config.Config{},
        sendWebhook: func(url string, payload []byte, secret string) error {
            calledURL = url
            calledBody = payload
            return nil
        },
    }
    rule := &db.Rule{Name: "webhook-rule", Actions: []db.Action{{Type: "webhook", Value: "https://example.com/hook"}}}
    msg := &imap.Message{From: []imap.Address{{Email: "test@example.com"}}, Subject: "test", Date: time.Now()}
    p.executeActions(rule, 1, msg, nil)
    if calledURL != "https://example.com/hook" {
        t.Errorf("expected https://example.com/hook, got %s", calledURL)
    }
    if !strings.Contains(string(calledBody), "webhook-rule") {
        t.Error("payload should contain rule name")
    }
    // Verify activity log
    entries, _, _ := p.logRepo.ListFiltered("", "", "", "", 10, 1)
    found := false
    for _, e := range entries {
        if e.ActionType == "webhook" && e.Status == "success" {
            found = true
            break
        }
    }
    if !found {
        t.Error("should have logged webhook success")
    }
}
```

- [ ] **Step 6: Update docs.html**

In `internal/web/templates/docs.html`, add to the Actions table (after the `forward` row):

```html
<tr><td><code>webhook</code></td><td>POST JSON payload to URL</td><td>Webhook URL</td></tr>
```

- [ ] **Step 7: Verify**

```bash
go test ./... && go vet ./...
```

Expected: all 12 packages pass.

- [ ] **Step 8: Commit**

```
git add -A
git commit -m "feat: add webhook action type — POST JSON to URL on rule match"
```

---

### Task 7: Bulk Apply Rules to Folder

**Files:**
- Modify: `internal/poller/poller.go` (add `ApplyToFolder` method)
- Modify: `internal/web/web.go` (add routes `POST /rules/apply` + `GET /rules/apply/status`)
- Modify: `internal/web/rules.go` (add `rulesApplyHandler`)
- Modify: `internal/mcp/server.go` (add `apply_rules_to_folder` tool)
- Modify: `internal/web/templates/rules_list.html` (add apply button/form)
- Test: `internal/poller/poller_test.go` (add TestApplyToFolder)
- Test: `internal/web/rules_test.go` (add TestRulesApplyHandler)

**Design summary:** `Poller.ApplyToFolder(folder, limit, search) (ApplyResult, error)` runs a synchronous sweep of the given folder. It uses the same `executeActions` as normal polling but in a targeted pass. Web handler starts this asynchronously (immediate 202 response), stores progress in a simple in-memory map keyed by a job ID, and a status endpoint returns current progress. MCP tool does the same synchronously.

- [ ] **Step 1: Add ApplyToFolder to Poller**

In `internal/poller/poller.go`, add:

```go
type ApplyResult struct {
    Processed int       `json:"processed"`
    Matched   int       `json:"matched"`
    Errors    int       `json:"errors"`
    Actions   int       `json:"actions"`
    StartedAt time.Time `json:"started_at"`
}

type ApplyStatus struct {
    Running   bool        `json:"running"`
    Result    ApplyResult `json:"result"`
    Error     string      `json:"error,omitempty"`
    Folder    string      `json:"folder"`
}

func (p *Poller) ApplyToFolder(folder string, limit int) (*ApplyResult, error) {
    if limit <= 0 || limit > 200 {
        limit = 50
    }
    rules, err := p.rulesRepo.List()
    if err != nil {
        return nil, fmt.Errorf("list rules: %w", err)
    }
    result := &ApplyResult{StartedAt: time.Now(), Processed: 0}
    var minUID uint32 = 0
    for result.Processed < limit {
        uids, err := p.imapClient.SearchMessages(folder, 1, minUID)
        if err != nil {
            return result, fmt.Errorf("search: %w", err)
        }
        if len(uids) == 0 {
            break
        }
        uid := uint32(uids[0])
        msg, err := p.imapClient.FetchMessage(uid)
        if err != nil {
            result.Errors++
            minUID = uid + 1
            continue
        }
        matched, captures, err := rules.Match(rules, msg, p.imapClient, p.timeLocation())
        if err != nil {
            result.Errors++
            minUID = uid + 1
            continue
        }
        result.Processed++
        if matched != nil {
            result.Matched++
            p.executeActions(matched, uid, msg, captures)
            result.Actions += len(matched.Actions)
        }
        minUID = uid + 1
    }
    return result, nil
}
```

- [ ] **Step 2: Add async job tracking for web**

In `internal/web/rules.go`, add a simple in-memory job store:

```go
var applyJobs sync.Map // map[string]*poller.ApplyStatus

func rulesApplyHandler(repo *db.RulesRepo, p *poller.Poller) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        folder := r.FormValue("folder")
        if folder == "" {
            http.Error(w, "folder is required", http.StatusBadRequest)
            return
        }
        limitStr := r.FormValue("limit")
        limit, _ := strconv.Atoi(limitStr)
        if limit <= 0 {
            limit = 50
        }
        jobID := fmt.Sprintf("%d", time.Now().UnixNano())
        status := &poller.ApplyStatus{Running: true, Folder: folder}
        applyJobs.Store(jobID, status)
        go func() {
            result, err := p.ApplyToFolder(folder, limit)
            if err != nil {
                status.Error = err.Error()
            } else {
                status.Result = *result
            }
            status.Running = false
            applyJobs.Store(jobID, status)
        }()
        w.Header().Set("Content-Type", "text/html")
        fmt.Fprintf(w, `<div hx-get="/rules/apply/status?id=%s" hx-trigger="every 2s" hx-swap="outerHTML">
            <p>Applying rules to <strong>%s</strong>... <span class="spinner"></span></p>
        </div>`, jobID, folder)
    }
}

func rulesApplyStatusHandler() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        jobID := r.URL.Query().Get("id")
        val, ok := applyJobs.Load(jobID)
        if !ok {
            http.Error(w, "job not found", http.StatusNotFound)
            return
        }
        status := val.(*poller.ApplyStatus)
        if status.Running {
            w.Header().Set("Content-Type", "text/html")
            fmt.Fprintf(w, `<div hx-get="/rules/apply/status?id=%s" hx-trigger="every 2s" hx-swap="outerHTML">
                <p>Processing... (%d matched so far)</p>
                <span class="spinner"></span>
            </div>`, jobID, status.Result.Matched)
            return
        }
        if status.Error != "" {
            w.Header().Set("Content-Type", "text/html")
            fmt.Fprintf(w, `<div class="toast error">Error: %s</div>`, status.Error)
            return
        }
        w.Header().Set("Content-Type", "text/html")
        fmt.Fprintf(w, `<div class="toast success">
            Done: %d processed, %d matched, %d actions, %d errors
        </div>`, status.Result.Processed, status.Result.Matched, status.Result.Actions, status.Result.Errors)
    }
}
```

Add `"sync"` and `"fmt"` to imports in rules.go.

- [ ] **Step 3: Register routes**

In `internal/web/web.go`, add:

```go
r.Post("/rules/apply", rulesApplyHandler(rulesRepo, p))
r.Get("/rules/apply/status", rulesApplyStatusHandler())
```

- [ ] **Step 4: Add UI button in rules_list.html**

In `internal/web/templates/rules_list.html`, after the search bar and before the rules table, add:

```html
<form hx-post="/rules/apply" hx-target="#apply-result" hx-swap="innerHTML" style="display:flex;gap:8px;align-items:center;margin-bottom:16px">
    <input name="folder" placeholder="Folder name" list="folder-list" required>
    <input name="limit" type="number" value="50" min="1" max="200" style="width:80px" title="Max messages">
    <button type="submit" class="btn">Apply Rules to Folder</button>
</form>
<div id="apply-result"></div>
```

- [ ] **Step 5: Add MCP tool**

In `internal/mcp/server.go`, after the `run_poll` tool, add:

```go
s.AddTool(mcp.NewTool("apply_rules_to_folder",
    mcp.WithDescription("Apply all enabled rules to messages in a folder"),
    mcp.WithString("folder", mcp.Required(), mcp.Description("IMAP folder name")),
    mcp.WithNumber("limit", mcp.Description("Max messages to process (default 50, max 200)")),
), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
    args := req.GetArguments()
    folder, _ := args["folder"].(string)
    if folder == "" {
        return mcp.NewToolResultError("folder is required"), nil
    }
    limit := 50
    if v, ok := args["limit"].(float64); ok && v > 0 {
        l := int(v)
        if l > 200 {
            return mcp.NewToolResultError("limit must not exceed 200"), nil
        }
        limit = l
    }
    result, err := p.ApplyToFolder(folder, limit)
    if err != nil {
        return mcp.NewToolResultError(err.Error()), nil
    }
    return resultJSON(result)
})
```

The `p` variable (Poller) is already in scope in the `New()` function where tools are registered.

- [ ] **Step 6: Write tests**

In `internal/poller/poller_test.go`:

```go
func TestApplyToFolder(t *testing.T) {
    database := db.NewTestDB(t)
    mock := &trackedMock{
        messages: map[uint32]*imap.Message{
            1: {Subject: "test", From: []imap.Address{{Email: "a@example.com"}}},
        },
        searchUIDs: []goimap.UID{1},
    }
    rulesRepo := db.NewRulesRepo(database)
    rulesRepo.Create(&db.Rule{
        Name: "test", Enabled: true, Priority: 1,
        Groups: []db.ConditionGroup{{Operator: "AND", Conditions: []db.Condition{{Field: "from", Operator: "contains", Value: "example"}}}},
        Actions: []db.Action{{Type: "mark_as_read", Value: ""}},
    })
    p := &Poller{
        imapClient: mock, rulesRepo: rulesRepo, logRepo: db.NewLogRepo(database),
        statsRepo: db.NewStatsRepo(database), settingsRepo: db.NewSettingsRepo(database),
        cfg: &config.Config{},
    }
    result, err := p.ApplyToFolder("INBOX", 10)
    if err != nil {
        t.Fatal(err)
    }
    if result.Processed != 1 {
        t.Errorf("expected 1 processed, got %d", result.Processed)
    }
    if result.Matched != 1 {
        t.Errorf("expected 1 matched, got %d", result.Matched)
    }
}
```

In `internal/web/rules_test.go`:

```go
func TestRulesApplyHandler(t *testing.T) {
    // POST /rules/apply with folder=INBOX&limit=10
    // Assert 200 response
}
```

- [ ] **Step 7: Verify**

```bash
go test ./... && go vet ./...
```

Expected: all 12 packages pass.

- [ ] **Step 8: Commit**

```
git add -A
git commit -m "feat: bulk apply rules to existing folders via web UI and MCP"
```

---

### Task 8: Final Docs + CHANGELOG Update

**Files:**
- Modify: `CHANGELOG.md` (add all 7 features under `[Unreleased]`)
- Modify: `internal/web/templates/docs.html` (add new routes: /metrics, webhook action, dry-run endpoint, bulk apply)
- Modify: `README.md` (add features to list)
- Modify: `AGENTS.md` (add prometheus.go to architecture)

- [ ] **Step 1: Update CHANGELOG.md**

Under `## [Unreleased]`, add:

```markdown
### Added
- CI test workflow — `go test` and `go vet` run on every push and PR to main
- Prometheus `/metrics` endpoint with counters (messages, rules, actions, errors, ticks), gauges (CPU, memory, uptime), and histogram (tick duration)
- Rule time/day scheduling — optional `schedule_days`, `schedule_start`, and `schedule_end` per rule, gated at match level
- Richer auto-reply templating — `[to]`, `[cc]`, `[rule_name]` variables, plus `[capture:name]` for regex named capture groups
- Web UI rule dry-run — test rules against synthetic or real messages with per-condition pass/fail breakdown
- Webhook action type — POST JSON payload (rule, subject, from, to, date, uid) to a configurable URL on rule match
- Bulk apply rules to existing folders — apply all enabled rules to messages in any IMAP folder via web UI or MCP
```

- [ ] **Step 2: Update docs.html sidebar**

Add to the API section:
```html
<a href="#api-apply">Bulk Apply</a>
<a href="#api-metrics">Metrics</a>
```

Add the endpoint documentation blocks:

After the `/health` endpoint block, add:

```html
<div class="endpoint">
    <h4 id="api-metrics">GET /metrics</h4>
    <p>Prometheus metrics endpoint (public). Returns counters, gauges, and histograms for system monitoring.</p>
    <table>
        <tr><th>Metric</th><th>Type</th><th>Description</th></tr>
        <tr><td><code>mailflow_messages_processed_total</code></td><td>Counter</td><td>Total messages processed</td></tr>
        <tr><td><code>mailflow_rules_matched_total</code></td><td>Counter</td><td>Rule matches (label: rule)</td></tr>
        <tr><td><code>mailflow_actions_total</code></td><td>Counter</td><td>Actions executed (labels: type, status)</td></tr>
        <tr><td><code>mailflow_errors_total</code></td><td>Counter</td><td>Processing errors</td></tr>
        <tr><td><code>mailflow_poller_ticks_total</code></td><td>Counter</td><td>Poller tick count</td></tr>
        <tr><td><code>mailflow_poller_tick_duration_seconds</code></td><td>Histogram</td><td>Tick duration distribution</td></tr>
        <tr><td><code>mailflow_cpu_percent</code></td><td>Gauge</td><td>Current CPU %</td></tr>
        <tr><td><code>mailflow_memory_bytes</code></td><td>Gauge</td><td>Current memory usage</td></tr>
        <tr><td><code>mailflow_uptime_seconds</code></td><td>Gauge</td><td>Process uptime</td></tr>
    </table>
</div>

<div class="endpoint">
    <h4 id="api-apply">POST /rules/apply</h4>
    <p>Apply all enabled rules to messages in a folder. Runs asynchronously; poll status at <code>GET /rules/apply/status?id=...</code>.</p>
    <table>
        <tr><th>Field</th><th>Type</th><th>Required</th><th>Description</th></tr>
        <tr><td>folder</td><td>string</td><td>Yes</td><td>IMAP folder name</td></tr>
        <tr><td>limit</td><td>number</td><td>No</td><td>Max messages (default 50, max 200)</td></tr>
    </table>
</div>

<div class="endpoint">
    <h4>POST /rules/{id}/test</h4>
    <p>Test a rule against synthetic inputs (from, to, cc, subject). Returns match result with per-condition breakdown.</p>
</div>
```

- [ ] **Step 3: Update docs.html Actions table**

Add the webhook row and update the auto_reply row to mention new template variables:

The webhook row was added in Task 6 Step 6. The auto_reply row should have updated template docs:

```html
<tr><td><code>auto_reply</code></td><td>Send templated reply to sender</td><td>Reply body (<code>[subject]</code>, <code>[from]</code>, <code>[date]</code>, <code>[to]</code>, <code>[cc]</code>, <code>[rule_name]</code>, <code>[capture:name]</code>)</td></tr>
```

- [ ] **Step 4: Update README.md features list**

Add these bullets to the Features section:

```markdown
- **Rule Scheduling** — optional time-of-day and day-of-week filter per rule
- **Regex Capture** — named groups from `matches_regex` conditions become template variables
- **Auto-Reply Templating** — `[to]`, `[cc]`, `[rule_name]`, and `[capture:name]` variables
- **Webhook Action** — POST JSON notification to any URL on rule match
- **Bulk Apply** — retroactively apply rules to existing mail in any folder
- **Prometheus Metrics** — `/metrics` endpoint with counters, gauges, and histogram
```

Remove the old auto-reply throttling bullet if present (it's from v0.8.0, keep it).

- [ ] **Step 5: Update AGENTS.md architecture**

Add `prometheus.go` to the web bullet:
```
- internal/web/ — chi router, auth, handlers, prometheus.go, embedded templates
```

- [ ] **Step 6: Verify**

```bash
go build ./... && go test ./... && go vet ./...
```

Expected: all 12 packages pass, build clean.

- [ ] **Step 7: Final verification and commit**

```bash
git add -A
git commit -m "docs: add changelog entries and documentation for 7 new features"
```

---

## Execution Order Summary

| Order | Task | Depends On | Independent? |
|-------|------|------------|--------------|
| 1 | CI Test Workflow | Nothing | Yes |
| 2 | Prometheus /metrics | Nothing | Yes |
| 3 | Rule Time/Day Scheduling | Nothing (but modifies engine.Match) | Yes |
| 4 | Richer Templating + Regex Capture | Task 3 (Match signature change) | No |
| 5 | Web UI Rule Dry-Run | Task 4 (EvaluateWithResults) | No |
| 6 | Webhook Action Type | Nothing (adds to poller cleanly) | Yes |
| 7 | Bulk Apply to Folder | Task 4 (Match signature) | No |
| 8 | Docs + CHANGELOG | All above | No |

Run in order: 1 → 2 → 3 → 4 → 5 → 6 → 7 → 8. Tasks 1, 2, and 6 can start in parallel if desired.
