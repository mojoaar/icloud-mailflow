package poller

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/mojoaar/icloud-mailflow/internal/contacts"
	"github.com/mojoaar/icloud-mailflow/internal/db"
	"github.com/mojoaar/icloud-mailflow/internal/imap"
	"github.com/mojoaar/icloud-mailflow/internal/rules"
)

type Poller struct {
	imapClient *imap.IMAPClient
	rulesRepo *db.RulesRepo
	collector *contacts.Collector
	interval  time.Duration
	source    string
	stopCh    chan struct{}
	wg        sync.WaitGroup
	running   bool
}

func NewPoller(imapClient *imap.IMAPClient, rulesRepo *db.RulesRepo, collector *contacts.Collector, intervalSec int, source string) *Poller {
	return &Poller{
		imapClient: imapClient,
		rulesRepo: rulesRepo,
		collector: collector,
		interval:  time.Duration(intervalSec) * time.Second,
		source:    source,
		stopCh:    make(chan struct{}),
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
	uids, err := p.imapClient.SearchMessages(p.source, 50)
	if err != nil {
		return fmt.Errorf("search: %w", err)
	}
	if len(uids) == 0 {
		return nil
	}
	slog.Debug("poller found messages", "count", len(uids), "folder", p.source)
	ruleList, err := p.rulesRepo.List()
	if err != nil {
		return fmt.Errorf("list rules: %w", err)
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
			slog.Error("rule matching error", "uid", uid, "error", err)
			continue
		}
		if matched != nil {
			slog.Debug("rule matched", "uid", uid, "rule", matched.Name)
			p.executeActions(matched, uint32(uid))
		}
	}
	return nil
}

func (p *Poller) executeActions(rule *db.Rule, uid uint32) {
	for _, action := range rule.Actions {
		switch action.Type {
		case "move_to_folder":
			if err := p.imapClient.MoveMessage(uid, action.Value); err != nil {
				slog.Error("move failed", "uid", uid, "dest", action.Value, "error", err)
			}
		default:
			slog.Warn("unknown action type", "type", action.Type)
		}
	}
}
