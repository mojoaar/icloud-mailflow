package poller

import (
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mojoaar/icloud-mailflow/internal/contacts"
	"github.com/mojoaar/icloud-mailflow/internal/db"
	"github.com/mojoaar/icloud-mailflow/internal/imap"
	"github.com/mojoaar/icloud-mailflow/internal/rules"
)

type Poller struct {
	imapClient imap.Client
	rulesRepo  *db.RulesRepo
	collector  *contacts.Collector
	logRepo    *db.LogRepo
	interval   time.Duration
	lastTick   atomic.Int64
	batchSize  int
	source     string
	stopCh     chan struct{}
	wg         sync.WaitGroup
	running    bool
	processing atomic.Bool
}

func NewPoller(imapClient imap.Client, rulesRepo *db.RulesRepo, collector *contacts.Collector, logRepo *db.LogRepo, batchSize int, intervalSec int, source string) *Poller {
	return &Poller{
		imapClient: imapClient,
		rulesRepo:  rulesRepo,
		collector:  collector,
		logRepo:    logRepo,
		batchSize:  batchSize,
		interval:   time.Duration(intervalSec) * time.Second,
		source:     source,
		stopCh:     make(chan struct{}),
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
	ruleList, err := p.rulesRepo.List()
	if err != nil {
		return fmt.Errorf("list rules: %w", err)
	}

	for processed := 0; processed < p.batchSize; processed++ {
		uids, err := p.imapClient.SearchMessages(p.source, 1)
		if err != nil {
			return fmt.Errorf("search: %w", err)
		}
		slog.Info("poller searching", "iteration", processed+1, "found", len(uids))
		if len(uids) == 0 {
			return nil
		}
		for _, uid := range uids {
			msg, err := p.imapClient.FetchMessage(uint32(uid))
			if err != nil {
				slog.Warn("poller failed to fetch message", "uid", uid, "error", err)
				continue
			}
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
