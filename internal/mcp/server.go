package mcp

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/mojoaar/icloud-mailflow/internal/contacts"
	"github.com/mojoaar/icloud-mailflow/internal/db"
	"github.com/mojoaar/icloud-mailflow/internal/imap"
	"github.com/mojoaar/icloud-mailflow/internal/poller"
	"github.com/mojoaar/icloud-mailflow/internal/rules"
)

func resultJSON(v any) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultJSON(v)
}

func New(d *sql.DB, imapClient imap.Client, p *poller.Poller, version string, collector *contacts.Collector, settingsRepo *db.SettingsRepo) *server.StreamableHTTPServer {
	rulesRepo := db.NewRulesRepo(d)
	logRepo := db.NewLogRepo(d)
	statsRepo := db.NewStatsRepo(d)
	contactsRepo := db.NewContactsRepo(d)

	s := server.NewMCPServer("mailflow", version)

	s.AddTool(mcp.NewTool("list_rules",
		mcp.WithDescription("List all filtering rules with conditions, groups and actions"),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		ruleList, err := rulesRepo.List()
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return resultJSON(map[string]any{"rules": ruleList})
	})

	s.AddTool(mcp.NewTool("get_rule",
		mcp.WithDescription("Get a single rule by ID with its conditions and actions"),
		mcp.WithNumber("id", mcp.Required(), mcp.Description("Rule ID")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := int64(req.GetArguments()["id"].(float64))
		rule, err := rulesRepo.Get(id)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return resultJSON(rule)
	})

	s.AddTool(mcp.NewTool("create_rule",
		mcp.WithDescription("Create a new filtering rule"),
		mcp.WithString("name", mcp.Required(), mcp.Description("Rule name")),
		mcp.WithString("conditions_json", mcp.Required(), mcp.Description("JSON with conditions and optional operator: {\"operator\":\"OR\",\"conditions\":[{\"field\":\"from\",\"operator\":\"contains\",\"value\":\"@example.com\"}]}")),
		mcp.WithString("actions_json", mcp.Required(), mcp.Description("JSON array of actions: [{\"type\":\"move_to_folder\",\"value\":\"Archive\"}]")),
		mcp.WithNumber("priority", mcp.Description("Priority (lower runs first, default 1)")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		name := args["name"].(string)
		priority := 1
		if v, ok := args["priority"]; ok {
			priority = int(v.(float64))
		}
		rule, err := parseRuleInput(name, priority, args["conditions_json"].(string), args["actions_json"].(string))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid input: %v", err)), nil
		}
		if err := rulesRepo.Create(rule); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		created, _ := rulesRepo.Get(rule.ID)
		return resultJSON(created)
	})

	s.AddTool(mcp.NewTool("update_rule",
		mcp.WithDescription("Update an existing rule"),
		mcp.WithNumber("id", mcp.Required(), mcp.Description("Rule ID")),
		mcp.WithString("name", mcp.Description("Rule name")),
		mcp.WithString("conditions_json", mcp.Description("JSON with conditions and optional operator")),
		mcp.WithString("actions_json", mcp.Description("JSON array of actions")),
		mcp.WithNumber("priority", mcp.Description("Priority (lower runs first)")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		id := int64(args["id"].(float64))

		existing, err := rulesRepo.Get(id)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if v, ok := args["name"]; ok {
			existing.Name = v.(string)
		}
		if v, ok := args["priority"]; ok {
			existing.Priority = int(v.(float64))
		}
		if condsJSON, ok := args["conditions_json"]; ok {
			actsJSON := `[]`
			if existing.Actions != nil {
				b, _ := json.Marshal(existing.Actions)
				actsJSON = string(b)
			}
			updated, err := parseRuleInput(existing.Name, existing.Priority, condsJSON.(string), actsJSON)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid conditions: %v", err)), nil
			}
			existing.Groups = updated.Groups
		}
		if actsJSON, ok := args["actions_json"]; ok {
			var actions []actionInput
			if err := json.Unmarshal([]byte(actsJSON.(string)), &actions); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid actions JSON: %v", err)), nil
			}
			existing.Actions = nil
			for _, a := range actions {
				existing.Actions = append(existing.Actions, db.Action{Type: a.Type, Value: a.Value})
			}
		}
		if err := rulesRepo.Update(existing); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		updated, _ := rulesRepo.Get(id)
		return resultJSON(updated)
	})

	s.AddTool(mcp.NewTool("delete_rule",
		mcp.WithDescription("Delete a rule by ID"),
		mcp.WithNumber("id", mcp.Required(), mcp.Description("Rule ID")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := int64(req.GetArguments()["id"].(float64))
		if err := rulesRepo.Delete(id); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText("rule deleted"), nil
	})

	s.AddTool(mcp.NewTool("check_email",
		mcp.WithDescription("Simulate rule matching against an email to see which rule would match"),
		mcp.WithString("from", mcp.Description("Sender email address")),
		mcp.WithString("to", mcp.Description("Recipient email address")),
		mcp.WithString("cc", mcp.Description("CC email address(es)")),
		mcp.WithString("subject", mcp.Description("Email subject")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		msg := &imap.Message{}
		if v, ok := args["from"]; ok {
			msg.From = []imap.Address{{Email: v.(string)}}
		}
		if v, ok := args["to"]; ok {
			msg.To = []imap.Address{{Email: v.(string)}}
		}
		if v, ok := args["cc"]; ok {
			msg.Cc = []imap.Address{{Email: v.(string)}}
		}
		if v, ok := args["subject"]; ok {
			msg.Subject = v.(string)
		}
		ruleList, err := rulesRepo.List()
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		matched, err := rules.Match(ruleList, msg, nil)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if matched == nil {
			return resultJSON(map[string]any{"matched": false})
		}
		return resultJSON(map[string]any{"matched": true, "rule": matched})
	})

	s.AddTool(mcp.NewTool("list_activity",
		mcp.WithDescription("List recent processing activity log entries"),
		mcp.WithNumber("limit", mcp.Description("Number of entries to return, default 50")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		limit := 50
		if v, ok := req.GetArguments()["limit"]; ok {
			limit = int(v.(float64))
		}
		entries, err := logRepo.ListRecent(limit)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return resultJSON(map[string]any{"entries": entries})
	})

	s.AddTool(mcp.NewTool("get_stats",
		mcp.WithDescription("Get processing statistics: total processed, rule hits, top senders, actions breakdown, daily volume"),
		mcp.WithNumber("days", mcp.Description("Days of daily volume to return, default 7")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		days := 7
		if v, ok := req.GetArguments()["days"]; ok {
			days = int(v.(float64))
		}
		total, _ := statsRepo.TotalProcessed()
		hits, _ := statsRepo.RuleHits()
		senders, _ := statsRepo.TopSenders(10)
		breakdown, _ := statsRepo.ActionsBreakdown()
		volume, _ := statsRepo.DailyVolume(days)
		return resultJSON(map[string]any{
			"total_processed":   total,
			"rule_hits":         hits,
			"top_senders":       senders,
			"actions_breakdown": breakdown,
			"daily_volume":      volume,
		})
	})

	s.AddTool(mcp.NewTool("run_poll",
		mcp.WithDescription("Manually trigger a poll cycle to process incoming mail immediately"),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if p == nil {
			return mcp.NewToolResultError("poller not available: IMAP not configured"), nil
		}
		if err := p.Tick(); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText("poll cycle completed"), nil
	})

	s.AddTool(mcp.NewTool("backup_rules",
		mcp.WithDescription("Export all rules as JSON (excludes catch-all)"),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		rules, err := rulesRepo.List()
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		type export struct {
			Name        string `json:"name"`
			Description string `json:"description,omitempty"`
			Priority    int    `json:"priority"`
			Enabled     bool   `json:"enabled"`
			Operator    string `json:"operator,omitempty"`
			Conditions  []struct {
				Field    string `json:"field"`
				Operator string `json:"operator"`
				Value    string `json:"value"`
			} `json:"conditions,omitempty"`
			Actions []struct {
				Type  string `json:"type"`
				Value string `json:"value"`
			} `json:"actions,omitempty"`
		}
		var result []export
		for _, r := range rules {
			if r.Name == "_catch_all" {
				continue
			}
			e := export{Name: r.Name, Description: r.Description, Priority: r.Priority, Enabled: r.Enabled}
			for _, g := range r.Groups {
				e.Operator = g.Operator
				for _, c := range g.Conditions {
					e.Conditions = append(e.Conditions, struct {
						Field    string `json:"field"`
						Operator string `json:"operator"`
						Value    string `json:"value"`
					}{c.Field, c.Operator, c.Value})
				}
			}
			for _, a := range r.Actions {
				e.Actions = append(e.Actions, struct {
					Type  string `json:"type"`
					Value string `json:"value"`
				}{a.Type, a.Value})
			}
			result = append(result, e)
		}
		return resultJSON(map[string]any{"rules": result})
	})

	s.AddTool(mcp.NewTool("list_folders",
		mcp.WithDescription("List available IMAP folders"),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if imapClient == nil {
			return mcp.NewToolResultError("IMAP not configured"), nil
		}
		folders, err := imapClient.ListFolders()
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		type folderSummary struct {
			Name  string `json:"name"`
			Path  string `json:"path"`
			Flags string `json:"flags"`
		}
		var result []folderSummary
		for _, f := range folders {
			result = append(result, folderSummary{f.Name, f.Path, f.Flags})
		}
		return resultJSON(map[string]any{"folders": result})
	})

	s.AddTool(mcp.NewTool("search_contacts",
		mcp.WithDescription("Search collected email contacts by name or email"),
		mcp.WithString("q", mcp.Required(), mcp.Description("Search query (matches name or email)")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		q := req.GetArguments()["q"].(string)
		contacts, err := contactsRepo.Search(q)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return resultJSON(map[string]any{"contacts": contacts})
	})

	s.AddTool(mcp.NewTool("list_contacts",
		mcp.WithDescription("List all collected email contacts"),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		contacts, err := contactsRepo.ListAll()
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return resultJSON(map[string]any{"contacts": contacts})
	})

	s.AddTool(mcp.NewTool("enable_rule",
		mcp.WithDescription("Enable a rule by ID"),
		mcp.WithNumber("rule_id", mcp.Required(), mcp.Description("Rule ID")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := int64(req.GetArguments()["rule_id"].(float64))
		rule, err := rulesRepo.Get(id)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		rule.Enabled = true
		if err := rulesRepo.Update(rule); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText("rule enabled"), nil
	})

	s.AddTool(mcp.NewTool("disable_rule",
		mcp.WithDescription("Disable a rule by ID"),
		mcp.WithNumber("rule_id", mcp.Required(), mcp.Description("Rule ID")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := int64(req.GetArguments()["rule_id"].(float64))
		rule, err := rulesRepo.Get(id)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		rule.Enabled = false
		if err := rulesRepo.Update(rule); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText("rule disabled"), nil
	})

	s.AddTool(mcp.NewTool("get_poller_status",
		mcp.WithDescription("Get poller status: running state, last tick, errors, health"),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if p == nil {
			return mcp.NewToolResultError("poller not running"), nil
		}
		return resultJSON(p.Status())
	})

	s.AddTool(mcp.NewTool("import_rules",
		mcp.WithDescription("Import rules from JSON array"),
		mcp.WithString("rules", mcp.Required(), mcp.Description("JSON array of rule objects in the same format as backup_rules output")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		rulesJSON := req.GetArguments()["rules"].(string)
		var input []struct {
			Name        string `json:"name"`
			Description string `json:"description,omitempty"`
			Priority    int    `json:"priority"`
			Enabled     bool   `json:"enabled"`
			Operator    string `json:"operator,omitempty"`
			Conditions  []struct {
				Field    string `json:"field"`
				Operator string `json:"operator"`
				Value    string `json:"value"`
			} `json:"conditions,omitempty"`
			Actions []struct {
				Type  string `json:"type"`
				Value string `json:"value"`
			} `json:"actions,omitempty"`
		}
		if err := json.Unmarshal([]byte(rulesJSON), &input); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		imported := 0
		for _, r := range input {
			op := r.Operator
			if op == "" {
				op = "OR"
			}
			if op != "AND" && op != "OR" {
				op = "OR"
			}
			rule := &db.Rule{Name: r.Name, Description: r.Description, Priority: r.Priority, Enabled: r.Enabled}
			if len(r.Conditions) > 0 {
				g := db.ConditionGroup{Operator: op}
				for _, c := range r.Conditions {
					g.Conditions = append(g.Conditions, db.Condition{Field: c.Field, Operator: c.Operator, Value: c.Value})
				}
				rule.Groups = []db.ConditionGroup{g}
			}
			for _, a := range r.Actions {
				rule.Actions = append(rule.Actions, db.Action{Type: a.Type, Value: a.Value})
			}
			if err := rulesRepo.Create(rule); err != nil {
				return mcp.NewToolResultError("import failed: " + err.Error()), nil
			}
			imported++
		}
		return resultJSON(map[string]int{"imported": imported})
	})

	s.AddTool(mcp.NewTool("clear_activity",
		mcp.WithDescription("Clear all activity logs (stats unaffected)"),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := logRepo.DeleteAll(); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText("activity log cleared"), nil
	})

	s.AddTool(mcp.NewTool("seed_contacts",
		mcp.WithDescription("Scan IMAP folders for contacts"),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if collector == nil {
			return mcp.NewToolResultError("contacts collector not configured"), nil
		}
		folders, err := db.NewFoldersRepo(d).List()
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		for _, f := range folders {
			if err := collector.SeedFromFolder(f.Name); err != nil {
				slog.Warn("seed contacts failed for folder", "folder", f.Name, "error", err)
			}
		}
		return resultJSON(map[string]int{"folders_scanned": len(folders)})
	})

	return server.NewStreamableHTTPServer(s)
}

func generateAPIKey() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func NewAuthMiddleware(mcpHandler http.Handler, settingsRepo *db.SettingsRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		enabled, _ := settingsRepo.Get("mcp_enabled")
		if enabled != "true" {
			http.Error(w, "MCP server is not enabled", http.StatusNotFound)
			return
		}
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			http.Error(w, "missing or invalid Authorization header", http.StatusUnauthorized)
			return
		}
		key, _ := settingsRepo.Get("mcp_api_key")
		if key == "" || auth != "Bearer "+key {
			http.Error(w, "invalid API key", http.StatusUnauthorized)
			return
		}
		mcpHandler.ServeHTTP(w, r)
	}
}

type conditionInput struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
}

type actionInput struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

func parseRuleInput(name string, priority int, condsJSON, actsJSON string) (*db.Rule, error) {
	var input struct {
		Operator   string           `json:"operator"`
		Conditions []conditionInput `json:"conditions"`
	}
	if err := json.Unmarshal([]byte(condsJSON), &input); err != nil {
		return nil, fmt.Errorf("conditions JSON: %w", err)
	}

	var actions []actionInput
	if actsJSON != "" {
		if err := json.Unmarshal([]byte(actsJSON), &actions); err != nil {
			return nil, fmt.Errorf("actions JSON: %w", err)
		}
	}

	op := input.Operator
	if op == "" {
		op = "OR"
	}
	op = strings.ToUpper(op)
	if op != "AND" && op != "OR" {
		op = "OR"
	}

	rule := &db.Rule{
		Name:     name,
		Priority: priority,
		Enabled:  true,
	}

	if len(input.Conditions) > 0 {
		g := db.ConditionGroup{Operator: op}
		for _, c := range input.Conditions {
			g.Conditions = append(g.Conditions, db.Condition{
				Field: c.Field, Operator: c.Operator, Value: c.Value,
			})
		}
		rule.Groups = []db.ConditionGroup{g}
	}

	for _, a := range actions {
		rule.Actions = append(rule.Actions, db.Action{Type: a.Type, Value: a.Value})
	}

	return rule, nil
}
