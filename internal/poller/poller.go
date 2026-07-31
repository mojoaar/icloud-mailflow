package poller

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mojoaar/icloud-mailflow/internal/contacts"
	"github.com/mojoaar/icloud-mailflow/internal/config"
	"github.com/mojoaar/icloud-mailflow/internal/db"
	"github.com/mojoaar/icloud-mailflow/internal/imap"
	"github.com/mojoaar/icloud-mailflow/internal/rules"
	"github.com/mojoaar/icloud-mailflow/internal/smtp"
)

type Poller struct {
	imapClient   imap.Client
	rulesRepo    *db.RulesRepo
	collector    *contacts.Collector
	logRepo      *db.LogRepo
	settingsRepo *db.SettingsRepo
	statsRepo    *db.StatsRepo
	cfg          *config.Config
	imapEmail    string
	interval     time.Duration
	lastTick     atomic.Int64
	batchSize    int
	source       string
	stopCh       chan struct{}
	wg           sync.WaitGroup
	running      bool
	processing   atomic.Bool
}

func NewPoller(imapClient imap.Client, rulesRepo *db.RulesRepo, collector *contacts.Collector, logRepo *db.LogRepo, settingsRepo *db.SettingsRepo, statsRepo *db.StatsRepo, cfg *config.Config, batchSize int, intervalSec int, source string) *Poller {
	return &Poller{
		imapClient:   imapClient,
		rulesRepo:    rulesRepo,
		collector:    collector,
		logRepo:      logRepo,
		settingsRepo: settingsRepo,
		statsRepo:    statsRepo,
		cfg:          cfg,
		batchSize:    batchSize,
		interval:     time.Duration(intervalSec) * time.Second,
		source:       source,
		stopCh:       make(chan struct{}),
	}
}

func (p *Poller) Start() {
	if p.running {
		return
	}
	p.running = true
	p.wg.Add(1)
	go p.loop()
	slog.Info("poller started", "interval", p.interval.String(), "source", p.source)
}

func (p *Poller) Stop() {
	if !p.running {
		return
	}
	p.running = false
	close(p.stopCh)
	p.wg.Wait()
	slog.Info("poller stopped")
}

func (p *Poller) Tick() error {
	return p.process()
}

func (p *Poller) LastTick() time.Time {
	ns := p.lastTick.Load()
	if ns == 0 {
		return time.Time{}
	}
	return time.Unix(0, ns)
}

func (p *Poller) loop() {
	defer p.wg.Done()
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()
	if err := p.process(); err != nil {
		slog.Error("poller initial tick failed", "error", err)
	}
	for {
		select {
		case <-ticker.C:
			if err := p.process(); err != nil {
				slog.Error("poller tick failed", "error", err)
			}
			p.checkBackup()
		case <-p.stopCh:
			return
		}
	}
}

func (p *Poller) process() error {
	if !p.processing.CompareAndSwap(false, true) {
		return nil
	}
	defer p.processing.Store(false)
	p.lastTick.Store(time.Now().UnixNano())
	slog.Debug("poller tick start", "source", p.source)
	ruleList, err := p.rulesRepo.List()
	if err != nil {
		return fmt.Errorf("list rules: %w", err)
	}
	slog.Debug("rules loaded", "count", len(ruleList))

	skipUIDs := map[uint32]bool{}
	highestSkipped := uint32(0)
	for processed := 0; processed < p.batchSize; processed++ {
		minUID := highestSkipped
		uids, err := p.imapClient.SearchMessages(p.source, 1, minUID)
		if err != nil {
			return fmt.Errorf("search: %w", err)
		}
		slog.Debug("search result", "found", len(uids), "minUID", minUID)
		if len(uids) == 0 {
			slog.Debug("folder empty, tick complete")
			return nil
		}
		remaining := uids[:0]
		for _, uid := range uids {
			if !skipUIDs[uint32(uid)] {
				remaining = append(remaining, uid)
			}
		}
		if len(remaining) == 0 {
			slog.Debug("all results already skipped, continuing")
			continue
		}
		for _, uid := range remaining {
			msg, err := p.imapClient.FetchMessage(uint32(uid))
			if err != nil {
				slog.Warn("poller failed to fetch message", "uid", uid, "error", err)
				continue
			}
			slog.Debug("fetched message", "uid", uid, "subj", msg.Subject)
			if p.collector != nil {
				p.collector.CollectFromMessage(msg)
			}
			matched, err := rules.Match(ruleList, msg)
			if err != nil {
				slog.Error("rule matching error", "uid", msg.UID, "error", err)
				continue
			}
			if matched != nil {
				slog.Debug("rule matched", "uid", uid, "rule", matched.Name)
				p.executeActions(matched, uint32(uid), msg)
			} else {
				u := uint32(uid)
				skipUIDs[u] = true
				if u >= highestSkipped {
					highestSkipped = u + 1
				}
				slog.Debug("no rule matched, skipping", "uid", uid)
			}
		}
	}
	return nil
}

func (p *Poller) executeActions(rule *db.Rule, uid uint32, msg *imap.Message) {
	from := ""
	if msg != nil && len(msg.From) > 0 {
		from = msg.From[0].Email
	}
	subject := ""
	if msg != nil {
		subject = msg.Subject
	}

	messageStatsDone := false

	logAction := func(actionUID uint32, action db.Action, status string) {
		if p.logRepo != nil {
			p.logRepo.Insert(&db.LogEntry{
				UID:         int64(actionUID),
				Subject:     subject,
				FromAddr:    from,
				RuleName:    rule.Name,
				ActionType:  action.Type,
				ActionValue: action.Value,
				Status:      status,
			})
		}
		if p.statsRepo != nil {
			if !messageStatsDone {
				p.statsRepo.IncrementStat("total", "processed")
				p.statsRepo.IncrementStat("rule_hit", rule.Name)
				if from != "" {
					p.statsRepo.IncrementStat("sender", from)
				}
				now := time.Now()
				p.statsRepo.IncrementStat("daily", now.Format("2006-01-02"))
				year, week := now.ISOWeek()
				p.statsRepo.IncrementStat("weekly", fmt.Sprintf("%d-W%02d", year, week))
				messageStatsDone = true
			}
			p.statsRepo.IncrementStat("action", action.Type)
			p.statsRepo.IncrementStat("status", status)
			if action.Type == "move_to_folder" && action.Value != "" {
				p.statsRepo.IncrementStat("folder", action.Value)
			}
		}
	}

	effectiveUID := uid
	destFolder := ""

	for _, action := range rule.Actions {
		switch action.Type {
		case "move_to_folder":
			if action.Value == "" {
				slog.Warn("move_to_folder action has empty value, skipping", "uid", effectiveUID, "rule", rule.Name)
				continue
			}
			newUID, err := p.imapClient.MoveMessage(effectiveUID, action.Value)
			if err != nil {
				slog.Error("move failed", "uid", effectiveUID, "dest", action.Value, "error", err)
				logAction(effectiveUID, action, "error")
			} else {
				logAction(effectiveUID, action, "success")
				effectiveUID = newUID
				destFolder = action.Value
				p.imapClient.SelectMailbox(destFolder)
			}
		case "mark_as_read":
			if err := p.imapClient.SetFlags(effectiveUID, []string{"\\Seen"}); err != nil {
				slog.Error("mark as read failed", "uid", effectiveUID, "error", err)
				logAction(effectiveUID, action, "error")
			} else {
				logAction(effectiveUID, action, "success")
			}
		case "mark_as_unread":
			if err := p.imapClient.RemoveFlags(effectiveUID, []string{"\\Seen"}); err != nil {
				slog.Error("mark as unread failed", "uid", effectiveUID, "error", err)
				logAction(effectiveUID, action, "error")
			} else {
				logAction(effectiveUID, action, "success")
			}
		case "set_flag":
			if action.Value == "" {
				slog.Warn("set_flag action has empty value, skipping", "uid", effectiveUID, "rule", rule.Name)
				continue
			}
			if err := p.imapClient.SetFlags(effectiveUID, []string{action.Value}); err != nil {
				slog.Error("set flag failed", "uid", effectiveUID, "flag", action.Value, "error", err)
				logAction(effectiveUID, action, "error")
			} else {
				logAction(effectiveUID, action, "success")
			}
		default:
			slog.Warn("unknown action type", "type", action.Type)
		}
	}
}

func (p *Poller) getIMAPEmail() string {
	if p.imapEmail != "" {
		return p.imapEmail
	}
	return p.cfg.IMAPEmail
}

func (p *Poller) BackupNow() error {
	if p.settingsRepo == nil || p.cfg == nil {
		return fmt.Errorf("backup: settings not configured")
	}
	rules, err := p.rulesRepo.List()
	if err != nil {
		return fmt.Errorf("backup: list rules: %w", err)
	}

	type backupRule struct {
		Name        string `json:"name"`
		Description string `json:"description,omitempty"`
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

	var export []backupRule
	for _, rule := range rules {
		if rule.Name == "_catch_all" {
			continue
		}
		br := backupRule{Name: rule.Name, Description: rule.Description}
		for _, g := range rule.Groups {
			br.Operator = g.Operator
			for _, c := range g.Conditions {
				br.Conditions = append(br.Conditions, struct {
					Field    string `json:"field"`
					Operator string `json:"operator"`
					Value    string `json:"value"`
				}{c.Field, c.Operator, c.Value})
			}
		}
		for _, a := range rule.Actions {
			br.Actions = append(br.Actions, struct {
				Type  string `json:"type"`
				Value string `json:"value"`
			}{a.Type, a.Value})
		}
		export = append(export, br)
	}

	jsonData, err := json.MarshalIndent(export, "", "  ")
	if err != nil {
		return fmt.Errorf("backup: marshal rules: %w", err)
	}

	from := p.getIMAPEmail()
	recipient, _ := p.settingsRepo.Get("backup_recipient")
	if recipient == "" {
		recipient = from
	}
	password := p.cfg.IMAPPassword

	ts := time.Now().Format("2006-01-02 15:04")
	subject := fmt.Sprintf("[Mailflow Backup] Rules - %s", ts)
	filename := fmt.Sprintf("icloud-mailflow-rules-%s.json", time.Now().Format("2006-01-02"))
	body := fmt.Sprintf("Mailflow rules backup from %s.\n\n%d rules exported.\n\nTo restore, download the JSON attachment and import it in Settings > Rules > Import.", ts, len(export))

	if err := smtp.Send(recipient, from, password, subject, body, smtp.Attachment{
		Name: filename, Data: jsonData,
	}); err != nil {
		return fmt.Errorf("backup: send email: %w", err)
	}

	p.settingsRepo.Set("last_backup", time.Now().Format(time.RFC3339))
	slog.Info("backup sent", "recipient", recipient, "rules", len(export))
	return nil
}

func (p *Poller) checkBackup() {
	if p.settingsRepo == nil {
		return
	}
	enabled, _ := p.settingsRepo.Get("backup_enabled")
	if enabled != "true" {
		return
	}

	lastStr, _ := p.settingsRepo.Get("last_backup")
	var lastBackup time.Time
	if lastStr != "" {
		lastBackup, _ = time.Parse(time.RFC3339, lastStr)
	}

	freq, _ := p.settingsRepo.Get("backup_frequency")
	if freq == "" {
		freq = "weekly"
	}
	var threshold time.Duration
	switch freq {
	case "daily":
		threshold = 24 * time.Hour
	case "monthly":
		threshold = 720 * time.Hour
	default:
		threshold = 168 * time.Hour
	}

	if time.Since(lastBackup) < threshold {
		return
	}

	if err := p.BackupNow(); err != nil {
		slog.Error("backup failed", "error", err)
	}
}
