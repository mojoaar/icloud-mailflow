package poller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"mime"
	"strings"
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
	foldersRepo  *db.FoldersRepo
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
	trashFolder  string
	mu                  sync.Mutex
	lastError           atomic.Value
	consecutiveFailures int
	backoff             time.Duration
	imapConnect         func() (imap.Client, error)
	lastTickDuration    time.Duration
}

func NewPoller(imapClient imap.Client, rulesRepo *db.RulesRepo, collector *contacts.Collector, logRepo *db.LogRepo, settingsRepo *db.SettingsRepo, statsRepo *db.StatsRepo, foldersRepo *db.FoldersRepo, cfg *config.Config, batchSize int, intervalSec int, source, imapEmail string, connectFn func() (imap.Client, error)) *Poller {
	return &Poller{
		imapClient:   imapClient,
		rulesRepo:    rulesRepo,
		collector:    collector,
		logRepo:      logRepo,
		settingsRepo: settingsRepo,
		statsRepo:    statsRepo,
		foldersRepo:  foldersRepo,
		cfg:          cfg,
		batchSize:    batchSize,
		interval:     time.Duration(intervalSec) * time.Second,
		source:       source,
		imapEmail:    imapEmail,
		imapConnect:  connectFn,
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

	start := time.Now()
	defer func() { p.lastTickDuration = time.Since(start) }()

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
			matched, err := rules.Match(ruleList, msg, p.imapClient)
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
	p.syncFolders()
	p.clearLastError()
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
		case "forward":
			if action.Value == "" {
				continue
			}
			raw, err := p.imapClient.FetchRawMessage(effectiveUID)
			if err != nil {
				logAction(effectiveUID, action, "error")
			} else {
				subject := "Fwd: " + msg.Subject
				mimeData := buildForwardMIME(string(raw), subject, p.imapEmail)
				if err := smtp.SendRaw(action.Value, p.imapEmail, p.cfg.IMAPPassword, mimeData); err != nil {
					logAction(effectiveUID, action, "error")
				} else {
					logAction(effectiveUID, action, "success")
				}
			}
		case "delete":
			trash, err := p.getTrashFolder()
			if err != nil {
				logAction(effectiveUID, action, "error")
			} else {
				newUID, err := p.imapClient.MoveMessage(effectiveUID, trash)
				if err != nil {
					logAction(effectiveUID, action, "error")
				} else {
					logAction(effectiveUID, action, "success")
					effectiveUID = newUID
					p.imapClient.SelectMailbox(trash)
				}
			}
		case "remove_flag":
			if action.Value == "" {
				continue
			}
			flag := action.Value
			if !strings.HasPrefix(flag, "\\") {
				if strings.EqualFold(flag, "seen") || strings.EqualFold(flag, "flagged") ||
					strings.EqualFold(flag, "answered") || strings.EqualFold(flag, "draft") {
					flag = "\\" + strings.ToUpper(flag[:1]) + strings.ToLower(flag[1:])
				} else {
					continue
				}
			}
			if err := p.imapClient.RemoveFlags(effectiveUID, []string{flag}); err != nil {
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

func (p *Poller) getTrashFolder() (string, error) {
	if p.trashFolder != "" {
		return p.trashFolder, nil
	}
	folders, err := p.imapClient.ListFolders()
	if err != nil {
		return "", err
	}
	for _, f := range folders {
		if strings.Contains(f.Flags, "\\Trash") {
			p.trashFolder = f.Name
			return p.trashFolder, nil
		}
	}
	for _, f := range folders {
		if strings.EqualFold(f.Name, "Deleted Messages") {
			p.trashFolder = f.Name
			return p.trashFolder, nil
		}
	}
	return "", fmt.Errorf("no trash folder found")
}

func buildForwardMIME(original, subject, from string) []byte {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("From: %s\r\n", from))
	buf.WriteString(fmt.Sprintf("Subject: %s\r\n", mime.QEncoding.Encode("utf-8", subject)))
	buf.WriteString(fmt.Sprintf("Date: %s\r\n", time.Now().Format(time.RFC1123Z)))
	buf.WriteString("MIME-Version: 1.0\r\n")
	boundary := fmt.Sprintf("mailflow-fwd-%d", time.Now().UnixNano())
	buf.WriteString(fmt.Sprintf("Content-Type: multipart/mixed; boundary=\"%s\"\r\n\r\n", boundary))
	buf.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	buf.WriteString("Content-Type: text/plain; charset=utf-8\r\n\r\n")
	buf.WriteString(fmt.Sprintf("Forwarded message from %s\r\n\r\n", from))
	buf.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	buf.WriteString("Content-Type: message/rfc822\r\n")
	buf.WriteString("Content-Disposition: attachment\r\n\r\n")
	buf.WriteString(original)
	buf.WriteString(fmt.Sprintf("\r\n--%s--\r\n", boundary))
	return buf.Bytes()
}

type PollerStatus struct {
	Active              bool          `json:"active"`
	Healthy             bool          `json:"healthy"`
	LastTick            time.Time     `json:"last_tick"`
	LastError           string        `json:"last_error"`
	LastDuration        time.Duration `json:"last_duration"`
	ProcessingMessages  bool          `json:"processing_messages"`
	ConsecutiveFailures int           `json:"consecutive_failures"`
}

func (p *Poller) Status() PollerStatus {
	lastErr := ""
	if v := p.lastError.Load(); v != nil {
		lastErr = v.(string)
	}
	p.mu.Lock()
	cf := p.consecutiveFailures
	p.mu.Unlock()
	return PollerStatus{
		Active:              p.running,
		Healthy:             cf == 0,
		LastTick:            time.Unix(0, p.lastTick.Load()),
		LastError:           lastErr,
		LastDuration:        p.lastTickDuration,
		ProcessingMessages:  p.processing.Load(),
		ConsecutiveFailures: cf,
	}
}

func (p *Poller) PollingHealthy() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.consecutiveFailures == 0
}

func (p *Poller) setLastError(err error) {
	slog.Error("action error", "error", err)
	p.lastError.Store(err.Error())
	p.mu.Lock()
	p.consecutiveFailures++
	cf := p.consecutiveFailures
	p.mu.Unlock()
	if cf > 2 && p.running && p.imapConnect != nil {
		go p.reconnect()
	}
}

func (p *Poller) clearLastError() {
	p.lastError.Store(nil)
	p.mu.Lock()
	p.consecutiveFailures = 0
	p.backoff = 0
	p.mu.Unlock()
}

func (p *Poller) syncFolders() {
	if p.foldersRepo == nil {
		return
	}
	folders, err := p.imapClient.ListFolders()
	if err != nil {
		slog.Warn("poller folder sync failed", "error", err)
		return
	}
	var dbFolders []db.Folder
	for _, f := range folders {
		dbFolders = append(dbFolders, db.Folder{
			Name:  f.Name,
			Path:  f.Path,
			Flags: f.Flags,
		})
	}
	if err := p.foldersRepo.Sync(dbFolders); err != nil {
		slog.Warn("poller folder sync db write failed", "error", err)
		return
	}
	slog.Debug("poller folder sync complete", "count", len(dbFolders))
}

func (p *Poller) reconnect() {
	p.mu.Lock()
	if p.backoff == 0 {
		p.backoff = 5 * time.Second
	}
	delay := p.backoff
	p.mu.Unlock()

	time.Sleep(delay)

	client, err := p.imapConnect()
	if err != nil {
		p.lastError.Store(err.Error())
		p.mu.Lock()
		p.consecutiveFailures++
		p.backoff *= 2
		if p.backoff > 60*time.Second {
			p.backoff = 60 * time.Second
		}
		p.mu.Unlock()
		return
	}
	p.imapClient = client
	p.clearLastError()
	slog.Info("IMAP reconnected")
}
