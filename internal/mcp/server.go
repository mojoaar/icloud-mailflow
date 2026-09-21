package mcp

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/mojoaar/icloud-mailflow/internal/config"
	"github.com/mojoaar/icloud-mailflow/internal/contacts"
	"github.com/mojoaar/icloud-mailflow/internal/db"
	"github.com/mojoaar/icloud-mailflow/internal/imap"
	"github.com/mojoaar/icloud-mailflow/internal/poller"
	"github.com/mojoaar/icloud-mailflow/internal/rules"
)

func resultJSON(v any) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultJSON(v)
}

var mcpLimiter = newMCPRateLimiter()

type mcpRateLimiter struct {
	mu      sync.Mutex
	entries map[string]*mcpRateEntry
}

type mcpRateEntry struct {
	count   int
	resetAt time.Time
}

func newMCPRateLimiter() *mcpRateLimiter {
	rl := &mcpRateLimiter{entries: map[string]*mcpRateEntry{}}
	go func() {
		for {
			time.Sleep(time.Minute)
			rl.cleanup()
		}
	}()
	return rl
}

func (rl *mcpRateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	for ip, e := range rl.entries {
		if now.After(e.resetAt) {
			delete(rl.entries, ip)
		}
	}
}

func (rl *mcpRateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	e, ok := rl.entries[ip]
	if !ok || now.After(e.resetAt) {
		rl.entries[ip] = &mcpRateEntry{count: 1, resetAt: now.Add(time.Minute)}
		return true
	}
	e.count++
	return e.count <= 100
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
		id, err := requiredIntArg(req.GetArguments(), "id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
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
		mcp.WithString("schedule_days", mcp.Description("Comma-separated days: mon,tue,wed,thu,fri,sat,sun (empty = always)")),
		mcp.WithString("schedule_start", mcp.Description("Start time HH:MM (empty = no start bound)")),
		mcp.WithString("schedule_end", mcp.Description("End time HH:MM (empty = no end bound)")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		name, err := requiredStringArg(args, "name")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		condsJSON, err := requiredStringArg(args, "conditions_json")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		actsJSON, err := requiredStringArg(args, "actions_json")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		priority := 1
		if v, ok := args["priority"]; ok {
			if f, ok := v.(float64); ok {
				priority = int(f)
			}
		}
		rule, err := parseRuleInput(name, priority, condsJSON, actsJSON)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid input: %v", err)), nil
		}
		if msgs := db.ValidateRule(rule); len(msgs) > 0 {
			return mcp.NewToolResultError("invalid rule: " + strings.Join(msgs, "; ")), nil
		}
		if v, ok := args["schedule_days"]; ok {
			rule.ScheduleDays = v.(string)
		}
		if v, ok := args["schedule_start"]; ok {
			rule.ScheduleStart = v.(string)
		}
		if v, ok := args["schedule_end"]; ok {
			rule.ScheduleEnd = v.(string)
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
		mcp.WithString("schedule_days", mcp.Description("Comma-separated days: mon,tue,wed,thu,fri,sat,sun (empty = always)")),
		mcp.WithString("schedule_start", mcp.Description("Start time HH:MM (empty = no start bound)")),
		mcp.WithString("schedule_end", mcp.Description("End time HH:MM (empty = no end bound)")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		id, err := requiredIntArg(args, "id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

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
		if v, ok := args["schedule_days"]; ok {
			existing.ScheduleDays = v.(string)
		}
		if v, ok := args["schedule_start"]; ok {
			existing.ScheduleStart = v.(string)
		}
		if v, ok := args["schedule_end"]; ok {
			existing.ScheduleEnd = v.(string)
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
		id, err := requiredIntArg(req.GetArguments(), "id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
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
		matched, _, err := rules.Match(ruleList, msg, nil, time.UTC)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if matched == nil {
			return resultJSON(map[string]any{"matched": false})
		}
		return resultJSON(map[string]any{"matched": true, "rule": matched})
	})

	s.AddTool(mcp.NewTool("list_activity",
		mcp.WithDescription("List activity log entries with optional filtering and pagination"),
		mcp.WithString("search", mcp.Description("Search term to match in subject, from address, or rule name")),
		mcp.WithString("rule", mcp.Description("Filter by rule name")),
		mcp.WithString("status", mcp.Description("Filter by status (success or error)")),
		mcp.WithNumber("page", mcp.Description("Page number (1-based, default 1)")),
		mcp.WithNumber("per_page", mcp.Description("Entries per page (default 50)")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		search := ""
		if v, ok := args["search"]; ok {
			search = v.(string)
		}
		rule := ""
		if v, ok := args["rule"]; ok {
			rule = v.(string)
		}
		status := ""
		if v, ok := args["status"]; ok {
			status = v.(string)
		}
		perPage := 50
		if v, ok := args["per_page"]; ok {
			perPage = int(v.(float64))
		}
		if perPage <= 0 {
			perPage = 50
		}
		if perPage > 500 {
			perPage = 500
		}
		page := 1
		if v, ok := args["page"]; ok {
			page = int(v.(float64))
		}
		if page < 1 {
			page = 1
		}
		offset := (page - 1) * perPage
		entries, total, err := logRepo.ListFiltered(perPage, offset, search, rule, status)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		totalPages := 1
		if perPage > 0 {
			totalPages = (total + perPage - 1) / perPage
		}
		return resultJSON(map[string]any{
			"entries":       entries,
			"total_entries": total,
			"total_pages":   totalPages,
			"page":          page,
		})
	})

	s.AddTool(mcp.NewTool("get_stats",
		mcp.WithDescription("Get processing statistics: total processed, rule hits, top senders, actions breakdown, errors, folder distribution, daily and weekly volume"),
		mcp.WithNumber("days", mcp.Description("Days of daily volume to return, default 7")),
		mcp.WithNumber("weeks", mcp.Description("Weeks of weekly volume to return, default 24")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		days := 7
		if v, ok := args["days"]; ok {
			days = int(v.(float64))
		}
		weeks := 24
		if v, ok := args["weeks"]; ok {
			weeks = int(v.(float64))
		}
		total, _ := statsRepo.TotalProcessed()
		hits, _ := statsRepo.RuleHits()
		senders, _ := statsRepo.TopSenders(20)
		breakdown, _ := statsRepo.ActionsBreakdown()
		volume, _ := statsRepo.DailyVolume(days)
		errors, _ := statsRepo.ErrorBreakdown()
		folders, _ := statsRepo.FolderDistribution()
		weekly, _ := statsRepo.WeeklyVolume(weeks)
		return resultJSON(map[string]any{
			"total_processed":     total,
			"rule_hits":           hits,
			"top_senders":         senders,
			"actions_breakdown":   breakdown,
			"daily_volume":        volume,
			"error_breakdown":     errors,
			"folder_distribution": folders,
			"weekly_volume":       weekly,
		})
	})

	s.AddTool(mcp.NewTool("run_poll",
		mcp.WithDescription("Manually trigger a poll cycle to process incoming mail immediately"),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if p == nil {
			return mcp.NewToolResultError("poller not available: IMAP not configured"), nil
		}
		started, err := p.TryTick()
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if !started {
			return mcp.NewToolResultText("poll already in progress"), nil
		}
		return mcp.NewToolResultText("poll cycle completed"), nil
	})

	s.AddTool(mcp.NewTool("backup_now",
		mcp.WithDescription("Manually trigger a rules backup email — exports all rules as JSON and emails them to the configured backup recipient"),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if p == nil {
			return mcp.NewToolResultError("poller not available: IMAP not configured"), nil
		}
		if err := p.BackupNow(); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText("backup completed and email sent"), nil
	})

	s.AddTool(mcp.NewTool("backup_rules",
		mcp.WithDescription("Export all rules as JSON (excludes catch-all)"),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		exported, err := rulesRepo.Export()
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return resultJSON(map[string]any{"rules": exported})
	})

	s.AddTool(mcp.NewTool("list_folders",
		mcp.WithDescription("List available IMAP folders"),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if imapClient == nil {
			return mcp.NewToolResultError("IMAP not configured"), nil
		}
		unlock := imap.LockSession(imapClient)
		defer unlock()
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
		q, err := requiredStringArg(req.GetArguments(), "q")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
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
		id, err := requiredIntArg(req.GetArguments(), "rule_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
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
		id, err := requiredIntArg(req.GetArguments(), "rule_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
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
		if p == nil {
			return mcp.NewToolResultError("poller not available: IMAP not configured"), nil
		}
		result, err := p.ApplyToFolder(folder, limit)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return resultJSON(result)
	})

	s.AddTool(mcp.NewTool("import_rules",
		mcp.WithDescription("Validate and import rules from a JSON array or the backup envelope. Invalid files are rejected; duplicate names are skipped."),
		mcp.WithString("rules", mcp.Required(), mcp.Description("JSON array of rule objects, or {\"rules\":[...]}, in the backup_rules format")),
		mcp.WithBoolean("dry_run", mcp.Description("Validate and preview without importing (default false)")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		rulesJSON, err := requiredStringArg(args, "rules")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if v, ok := args["dry_run"].(bool); ok && v {
			preview, err := rulesRepo.PreviewImport([]byte(rulesJSON))
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return resultJSON(preview)
		}
		report, err := rulesRepo.ImportWithReport([]byte(rulesJSON), db.ImportOptions{})
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return resultJSON(report)
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
		unlock := imap.LockSession(imapClient)
		defer unlock()
		for _, f := range folders {
			if err := collector.SeedFromFolder(f.Name); err != nil {
				slog.Warn("seed contacts failed for folder", "folder", f.Name, "error", err)
			}
		}
		return resultJSON(map[string]int{"folders_scanned": len(folders)})
	})

	s.AddTool(mcp.NewTool("toggle_contacts_collection",
		mcp.WithDescription("Enable or disable automatic contacts collection during email processing"),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		enabled, _ := settingsRepo.Get("contacts_collection_enabled")
		if enabled == "false" {
			settingsRepo.Set("contacts_collection_enabled", "true")
			return mcp.NewToolResultText("contacts collection enabled"), nil
		}
		settingsRepo.Set("contacts_collection_enabled", "false")
		return mcp.NewToolResultText("contacts collection disabled"), nil
	})

	s.AddTool(mcp.NewTool("wipe_contacts",
		mcp.WithDescription("Delete all collected contacts"),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := contactsRepo.DeleteAll(); err != nil {
			return mcp.NewToolResultError("failed to wipe contacts: " + err.Error()), nil
		}
		return mcp.NewToolResultText("all contacts wiped"), nil
	})

	s.AddTool(mcp.NewTool("health",
		mcp.WithDescription("Get system health: status, version, IMAP connection, poller state, and summary stats"),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		status := "ok"
		health := map[string]any{
			"version": version,
		}

		if err := d.Ping(); err != nil {
			health["db"] = "error"
			status = "degraded"
		} else {
			health["db"] = "ok"
		}

		if imapClient != nil {
			health["imap"] = "connected"
		} else {
			health["imap"] = "not_configured"
		}

		if p != nil {
			ps := p.Status()
			health["poller"] = map[string]any{
				"active":               ps.Active,
				"healthy":              ps.Healthy,
				"last_tick":            ps.LastTick.Format("2006-01-02T15:04:05Z07:00"),
				"last_duration_ms":     ps.LastDuration.Milliseconds(),
				"consecutive_failures": ps.ConsecutiveFailures,
			}
			if !ps.Healthy {
				status = "degraded"
			}
		}

		total, _ := statsRepo.TotalProcessed()
		contactCount, _ := contactsRepo.Count()
		rules, _ := rulesRepo.List()
		health["stats"] = map[string]any{
			"total_processed": total,
			"contacts_count":  contactCount,
			"rules_count":     len(rules),
		}

		health["status"] = status

		return resultJSON(health)
	})

	s.AddTool(mcp.NewTool("get_settings",
		mcp.WithDescription("Get all non-sensitive settings (passwords, hashes, and API keys excluded)"),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if settingsRepo == nil {
			return mcp.NewToolResultError("settings not available"), nil
		}
		all, err := settingsRepo.GetAll()
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		sensitive := map[string]bool{
			"admin_password_hash": true,
			"imap_password":       true,
			"mcp_api_key":         true,
		}
		safeSettings := map[string]string{}
		for k, v := range all {
			if !sensitive[k] {
				safeSettings[k] = v
			}
		}
		return resultJSON(map[string]any{"settings": safeSettings})
	})

	s.AddTool(mcp.NewTool("update_settings",
		mcp.WithDescription("Update settings. Pass only the settings you want to change as key-value pairs."),
		mcp.WithString("source_folder", mcp.Description("IMAP folder to poll for new mail")),
		mcp.WithString("poll_interval", mcp.Description("Poll interval in seconds (minimum 60)")),
		mcp.WithString("poll_batch", mcp.Description("Max messages per poll (1-200)")),
		mcp.WithString("log_keep", mcp.Description("Log retention count (minimum 100)")),
		mcp.WithString("timezone", mcp.Description("Timezone, e.g. UTC or Europe/Copenhagen")),
		mcp.WithString("backup_enabled", mcp.Description("Set to 'true' or 'false'")),
		mcp.WithString("backup_frequency", mcp.Description("daily, weekly, or monthly")),
		mcp.WithString("backup_recipient", mcp.Description("Email address for backup recipient")),
		mcp.WithString("mcp_enabled", mcp.Description("Set to 'true' or 'false'")),
		mcp.WithString("contacts_collection_enabled", mcp.Description("Set to 'true' or 'false'")),
		mcp.WithString("font_mono", mcp.Description("Set to 'true' or 'false'")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if settingsRepo == nil {
			return mcp.NewToolResultError("settings not available"), nil
		}
		args := req.GetArguments()
		writable := map[string]bool{
			"source_folder": true, "poll_interval": true, "poll_batch": true, "log_keep": true,
			"timezone": true, "backup_enabled": true, "backup_frequency": true,
			"backup_recipient": true, "mcp_enabled": true, "contacts_collection_enabled": true,
			"font_mono": true,
		}
		validBool := map[string]bool{"true": true, "false": true}
		validFreq := map[string]bool{"daily": true, "weekly": true, "monthly": true}

		updated := 0
		var errors []string
		for k, v := range args {
			if !writable[k] {
				continue
			}
			str, ok := v.(string)
			if !ok {
				errors = append(errors, k+": value must be a string")
				continue
			}
			switch k {
			case "timezone":
				if !config.ValidTimezone(str) {
					errors = append(errors, fmt.Sprintf("%s: unknown timezone %q", k, str))
					continue
				}
			case "backup_enabled", "mcp_enabled", "contacts_collection_enabled", "font_mono":
				if !validBool[str] {
					errors = append(errors, k+": must be 'true' or 'false'")
					continue
				}
			case "backup_frequency":
				if !validFreq[str] {
					errors = append(errors, k+": must be daily, weekly, or monthly")
					continue
				}
			case "poll_interval":
				n, e := strconv.Atoi(str)
				if e != nil || config.ValidatePollInterval(n) != nil {
					errors = append(errors, k+": must be integer >= 60")
					continue
				}
			case "poll_batch":
				n, e := strconv.Atoi(str)
				if e != nil || config.ValidatePollBatch(n) != nil {
					errors = append(errors, k+": must be integer 1-200")
					continue
				}
			case "log_keep":
				n, e := strconv.Atoi(str)
				if e != nil || config.ValidateLogKeep(n) != nil {
					errors = append(errors, k+": must be integer >= 100")
					continue
				}
			}
			if err := settingsRepo.Set(k, str); err != nil {
				errors = append(errors, fmt.Sprintf("%s: %v", k, err))
				continue
			}
			updated++
		}
		result := map[string]any{"updated": updated}
		if len(errors) > 0 {
			result["errors"] = errors
		}
		return resultJSON(result)
	})

	return server.NewStreamableHTTPServer(s)
}

func clientIP(r *http.Request) string {
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

func NewAuthMiddleware(mcpHandler http.Handler, settingsRepo *db.SettingsRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		enabled, _ := settingsRepo.Get("mcp_enabled")
		if enabled != "true" {
			http.Error(w, "MCP server is not enabled", http.StatusNotFound)
			return
		}
		if !mcpLimiter.allow(clientIP(r)) {
			http.Error(w, "too many requests", http.StatusTooManyRequests)
			return
		}
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			http.Error(w, "missing or invalid Authorization header", http.StatusUnauthorized)
			return
		}
		key, _ := settingsRepo.Get("mcp_api_key")
		if key == "" || subtle.ConstantTimeCompare([]byte(auth), []byte("Bearer "+key)) != 1 {
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
			if c.Operator == "matches_regex" {
				if _, err := regexp.Compile(c.Value); err != nil {
					return nil, fmt.Errorf("invalid regex %q: %w", c.Value, err)
				}
			}
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

func requiredIntArg(args map[string]any, key string) (int64, error) {
	v, ok := args[key]
	if !ok {
		return 0, fmt.Errorf("%s is required", key)
	}
	f, ok := v.(float64)
	if !ok {
		return 0, fmt.Errorf("%s must be a number", key)
	}
	return int64(f), nil
}

func requiredStringArg(args map[string]any, key string) (string, error) {
	v, ok := args[key]
	if !ok {
		return "", fmt.Errorf("%s is required", key)
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("%s must be a string", key)
	}
	return s, nil
}
