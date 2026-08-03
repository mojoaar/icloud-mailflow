package poller

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	goimap "github.com/emersion/go-imap/v2"

	"github.com/mojoaar/icloud-mailflow/internal/config"
	"github.com/mojoaar/icloud-mailflow/internal/contacts"
	"github.com/mojoaar/icloud-mailflow/internal/db"
	"github.com/mojoaar/icloud-mailflow/internal/imap"
	"github.com/mojoaar/icloud-mailflow/internal/smtp"
)

type trackedMock struct {
	mu               sync.Mutex
	searchUIDs       []goimap.UID
	messages         map[uint32]*imap.Message
	searchErr        error
	fetchErr         error
	moveCalls        []moveCall
	setFlagsCalls    []setFlagsCall
	removeFlagsCalls []removeFlagsCall
	searchFolders    []string
	rawMessages      map[uint32][]byte
	messageBodies    map[uint32]string
	messageHeaders   map[uint32]map[string]string
	folders          []imap.Folder
}

type moveCall struct {
	UID  uint32
	Dest string
}

type setFlagsCall struct {
	UID   uint32
	Flags []string
}

type removeFlagsCall struct {
	UID   uint32
	Flags []string
}

func (m *trackedMock) SearchMessages(folder string, limit int, minUID uint32) ([]goimap.UID, error) {
	m.mu.Lock()
	m.searchFolders = append(m.searchFolders, folder)
	m.mu.Unlock()
	for len(m.searchUIDs) > 0 {
		uid := m.searchUIDs[0]
		if uint32(uid) < minUID {
			m.searchUIDs = m.searchUIDs[1:]
			continue
		}
		m.searchUIDs = m.searchUIDs[1:]
		return []goimap.UID{uid}, m.searchErr
	}
	return nil, m.searchErr
}

func (m *trackedMock) FetchMessage(uid uint32) (*imap.Message, error) {
	if m.fetchErr != nil {
		return nil, m.fetchErr
	}
	if msg, ok := m.messages[uid]; ok {
		return msg, nil
	}
	return &imap.Message{UID: uid}, nil
}

func (m *trackedMock) FetchMessages(uids []goimap.UID) ([]*imap.Message, error) {
	if m.fetchErr != nil {
		return nil, m.fetchErr
	}
	msgs := make([]*imap.Message, len(uids))
	for i, uid := range uids {
		if msg, ok := m.messages[uint32(uid)]; ok {
			msgs[i] = msg
		} else {
			msgs[i] = &imap.Message{UID: uint32(uid)}
		}
	}
	return msgs, nil
}

func (m *trackedMock) MoveMessage(uid uint32, dest string) (uint32, error) {
	m.mu.Lock()
	m.moveCalls = append(m.moveCalls, moveCall{UID: uid, Dest: dest})
	m.mu.Unlock()
	return uid + 100, nil
}

func (m *trackedMock) SetFlags(uid uint32, flags []string) error {
	m.mu.Lock()
	m.setFlagsCalls = append(m.setFlagsCalls, setFlagsCall{UID: uid, Flags: flags})
	m.mu.Unlock()
	return nil
}

func (m *trackedMock) SelectMailbox(name string) error { return nil }

func (m *trackedMock) RemoveFlags(uid uint32, flags []string) error {
	m.mu.Lock()
	m.removeFlagsCalls = append(m.removeFlagsCalls, removeFlagsCall{UID: uid, Flags: flags})
	m.mu.Unlock()
	return nil
}

func (m *trackedMock) CreateFolder(name string) error { return nil }

func (m *trackedMock) ListFolders() ([]imap.Folder, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.folders, nil
}

func (m *trackedMock) FetchMessageHeader(uid uint32, headerName string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.messageHeaders == nil {
		return "", nil
	}
	hdrs, ok := m.messageHeaders[uid]
	if !ok {
		return "", nil
	}
	return hdrs[headerName], nil
}

func (m *trackedMock) FetchMessageBody(uid uint32) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.messageBodies == nil {
		return "", nil
	}
	return m.messageBodies[uid], nil
}

func (m *trackedMock) FetchRawMessage(uid uint32) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.rawMessages == nil {
		return nil, nil
	}
	return m.rawMessages[uid], nil
}

func openPollerTestDB(t *testing.T) (*db.RulesRepo, *db.ContactsRepo) {
	t.Helper()
	database := db.NewTestDB(t)
	return db.NewRulesRepo(database), db.NewContactsRepo(database)
}

func testRule(t *testing.T, repo *db.RulesRepo, name string, enabled bool, dest string) *db.Rule {
	t.Helper()
	r := &db.Rule{Name: name, Enabled: enabled}
	if dest != "" {
		r.Actions = []db.Action{{Type: "move_to_folder", Value: dest}}
	}
	if err := repo.Create(r); err != nil {
		t.Fatalf("Create rule: %v", err)
	}
	return r
}

func TestProcessNoMessages(t *testing.T) {
	rulesRepo, contactsRepo := openPollerTestDB(t)
	mock := &trackedMock{searchUIDs: []goimap.UID{}}
	collector := contacts.NewCollector(contactsRepo, mock)
	p := NewPoller(mock, rulesRepo, collector, nil, nil, nil, nil, nil, 50, 60, "INBOX", "", nil)

	if err := p.process(); err != nil {
		t.Fatalf("process: %v", err)
	}

	mock.mu.Lock()
	if len(mock.moveCalls) != 0 {
		t.Fatal("expected no move calls")
	}
	mock.mu.Unlock()
}

func TestProcessMessageMatchesRule(t *testing.T) {
	rulesRepo, contactsRepo := openPollerTestDB(t)
	testRule(t, rulesRepo, "move-to-trash", true, "Trash")

	mock := &trackedMock{
		searchUIDs: []goimap.UID{1},
		messages: map[uint32]*imap.Message{
			1: {UID: 1, Subject: "test"},
		},
	}
	collector := contacts.NewCollector(contactsRepo, mock)
	p := NewPoller(mock, rulesRepo, collector, nil, nil, nil, nil, nil, 50, 60, "INBOX", "", nil)

	if err := p.process(); err != nil {
		t.Fatalf("process: %v", err)
	}

	mock.mu.Lock()
	if len(mock.moveCalls) != 1 {
		t.Fatalf("expected 1 move call, got %d", len(mock.moveCalls))
	}
	if mock.moveCalls[0].UID != 1 || mock.moveCalls[0].Dest != "Trash" {
		t.Fatalf("expected move uid=1 dest=Trash, got uid=%d dest=%s", mock.moveCalls[0].UID, mock.moveCalls[0].Dest)
	}
	mock.mu.Unlock()
}

func TestProcessDisabledRuleSkipped(t *testing.T) {
	rulesRepo, contactsRepo := openPollerTestDB(t)
	testRule(t, rulesRepo, "disabled-rule", false, "Trash")

	mock := &trackedMock{
		searchUIDs: []goimap.UID{1},
		messages: map[uint32]*imap.Message{
			1: {UID: 1, Subject: "test"},
		},
	}
	collector := contacts.NewCollector(contactsRepo, mock)
	p := NewPoller(mock, rulesRepo, collector, nil, nil, nil, nil, nil, 50, 60, "INBOX", "", nil)

	if err := p.process(); err != nil {
		t.Fatalf("process: %v", err)
	}

	mock.mu.Lock()
	if len(mock.moveCalls) != 0 {
		t.Fatal("expected no move calls for disabled rule")
	}
	mock.mu.Unlock()
}

func TestProcessSearchError(t *testing.T) {
	rulesRepo, contactsRepo := openPollerTestDB(t)
	mock := &trackedMock{searchErr: errors.New("connection refused")}
	collector := contacts.NewCollector(contactsRepo, mock)
	p := NewPoller(mock, rulesRepo, collector, nil, nil, nil, nil, nil, 50, 60, "INBOX", "", nil)

	err := p.process()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestProcessFetchErrorContinues(t *testing.T) {
	rulesRepo, contactsRepo := openPollerTestDB(t)
	testRule(t, rulesRepo, "move-to-trash", true, "Trash")

	mock := &trackedMock{
		searchUIDs: []goimap.UID{1, 2},
		fetchErr:   errors.New("fetch failed"),
	}
	collector := contacts.NewCollector(contactsRepo, mock)
	p := NewPoller(mock, rulesRepo, collector, nil, nil, nil, nil, nil, 50, 60, "INBOX", "", nil)

	if err := p.process(); err != nil {
		t.Fatalf("process should not return error on fetch failure: %v", err)
	}

	mock.mu.Lock()
	if len(mock.moveCalls) != 0 {
		t.Fatal("expected no move calls since fetch failed")
	}
	mock.mu.Unlock()
}

func TestProcessNilCollector(t *testing.T) {
	rulesRepo, _ := openPollerTestDB(t)
	testRule(t, rulesRepo, "move-to-trash", true, "Trash")

	mock := &trackedMock{
		searchUIDs: []goimap.UID{1},
		messages: map[uint32]*imap.Message{
			1: {UID: 1, Subject: "test"},
		},
	}
	p := NewPoller(mock, rulesRepo, nil, nil, nil, nil, nil, nil, 50, 60, "INBOX", "", nil)

	if err := p.process(); err != nil {
		t.Fatalf("process with nil collector: %v", err)
	}
}

func TestProcessSearchesSourceFolder(t *testing.T) {
	rulesRepo, contactsRepo := openPollerTestDB(t)
	mock := &trackedMock{searchUIDs: []goimap.UID{}}
	collector := contacts.NewCollector(contactsRepo, mock)
	p := NewPoller(mock, rulesRepo, collector, nil, nil, nil, nil, nil, 50, 60, "Archive", "", nil)

	p.process()

	mock.mu.Lock()
	if len(mock.searchFolders) != 1 || mock.searchFolders[0] != "Archive" {
		t.Fatalf("expected search on 'Archive', got %v", mock.searchFolders)
	}
	mock.mu.Unlock()
}

func TestExecuteActionsMoveToFolder(t *testing.T) {
	mock := &trackedMock{}
	p := &Poller{imapClient: mock}

	rule := &db.Rule{
		Name: "test",
		Actions: []db.Action{
			{Type: "move_to_folder", Value: "Archive"},
		},
	}
	p.executeActions(rule, 42, nil, nil)

	mock.mu.Lock()
	if len(mock.moveCalls) != 1 || mock.moveCalls[0].UID != 42 || mock.moveCalls[0].Dest != "Archive" {
		t.Fatalf("expected move uid=42 dest=Archive, got %+v", mock.moveCalls)
	}
	mock.mu.Unlock()
}

func TestExecuteActionsUnknownType(t *testing.T) {
	mock := &trackedMock{}
	p := &Poller{imapClient: mock}

	rule := &db.Rule{
		Name: "test",
		Actions: []db.Action{
			{Type: "unknown_action", Value: "data"},
		},
	}
	p.executeActions(rule, 42, nil, nil)

	mock.mu.Lock()
	if len(mock.moveCalls) != 0 {
		t.Fatal("expected no move calls for unknown action type")
	}
	mock.mu.Unlock()
}

func TestStartStop(t *testing.T) {
	rulesRepo, contactsRepo := openPollerTestDB(t)
	mock := &trackedMock{searchUIDs: []goimap.UID{}}
	collector := contacts.NewCollector(contactsRepo, mock)
	p := NewPoller(mock, rulesRepo, collector, nil, nil, nil, nil, nil, 50, 1, "INBOX", "", nil)

	p.Start()
	time.Sleep(50 * time.Millisecond)
	p.Stop()
}

func TestDoubleStart(t *testing.T) {
	rulesRepo, contactsRepo := openPollerTestDB(t)
	mock := &trackedMock{searchUIDs: []goimap.UID{}}
	collector := contacts.NewCollector(contactsRepo, mock)
	p := NewPoller(mock, rulesRepo, collector, nil, nil, nil, nil, nil, 50, 1, "INBOX", "", nil)

	p.Start()
	p.Start()
	p.Stop()
}

func TestDoubleStop(t *testing.T) {
	rulesRepo, contactsRepo := openPollerTestDB(t)
	mock := &trackedMock{searchUIDs: []goimap.UID{}}
	collector := contacts.NewCollector(contactsRepo, mock)
	p := NewPoller(mock, rulesRepo, collector, nil, nil, nil, nil, nil, 50, 1, "INBOX", "", nil)

	p.Start()
	time.Sleep(30 * time.Millisecond)
	p.Stop()
	p.Stop()
}

func TestExecuteActionsMarkAsRead(t *testing.T) {
	mock := &trackedMock{}
	p := &Poller{imapClient: mock}

	rule := &db.Rule{
		Name: "test",
		Actions: []db.Action{
			{Type: "mark_as_read"},
		},
	}
	p.executeActions(rule, 42, nil, nil)

	mock.mu.Lock()
	if len(mock.setFlagsCalls) != 1 {
		t.Fatalf("expected 1 SetFlags call, got %d", len(mock.setFlagsCalls))
	}
	if mock.setFlagsCalls[0].UID != 42 {
		t.Fatalf("expected uid 42, got %d", mock.setFlagsCalls[0].UID)
	}
	if len(mock.setFlagsCalls[0].Flags) != 1 || mock.setFlagsCalls[0].Flags[0] != "\\Seen" {
		t.Fatalf("expected \\Seen, got %v", mock.setFlagsCalls[0].Flags)
	}
	mock.mu.Unlock()
}

func TestExecuteActionsSetFlag(t *testing.T) {
	mock := &trackedMock{}
	p := &Poller{imapClient: mock}

	rule := &db.Rule{
		Name: "test",
		Actions: []db.Action{
			{Type: "set_flag", Value: "\\Flagged"},
		},
	}
	p.executeActions(rule, 7, nil, nil)

	mock.mu.Lock()
	if len(mock.setFlagsCalls) != 1 || mock.setFlagsCalls[0].Flags[0] != "\\Flagged" {
		t.Fatalf("expected \\Flagged, got %v", mock.setFlagsCalls)
	}
	mock.mu.Unlock()
}

func TestExecuteActionsMultipleActions(t *testing.T) {
	mock := &trackedMock{}
	p := &Poller{imapClient: mock}

	rule := &db.Rule{
		Name: "test",
		Actions: []db.Action{
			{Type: "mark_as_read"},
			{Type: "move_to_folder", Value: "Archive"},
		},
	}
	p.executeActions(rule, 10, nil, nil)

	mock.mu.Lock()
	if len(mock.setFlagsCalls) != 1 {
		t.Fatalf("expected 1 SetFlags, got %d", len(mock.setFlagsCalls))
	}
	if len(mock.moveCalls) != 1 {
		t.Fatalf("expected 1 MoveMessage, got %d", len(mock.moveCalls))
	}
	if mock.moveCalls[0].Dest != "Archive" {
		t.Fatalf("expected Archive, got %s", mock.moveCalls[0].Dest)
	}
	mock.mu.Unlock()
}

func TestExecuteActionsMoveToFolderEmptyValue(t *testing.T) {
	mock := &trackedMock{}
	p := &Poller{imapClient: mock}

	rule := &db.Rule{
		Name: "test",
		Actions: []db.Action{
			{Type: "move_to_folder", Value: ""},
		},
	}
	p.executeActions(rule, 42, nil, nil)

	mock.mu.Lock()
	if len(mock.moveCalls) != 0 {
		t.Fatal("expected no move calls for empty destination")
	}
	mock.mu.Unlock()
}

func TestExecuteActionsSetFlagEmptyValue(t *testing.T) {
	mock := &trackedMock{}
	p := &Poller{imapClient: mock}

	rule := &db.Rule{
		Name: "test",
		Actions: []db.Action{
			{Type: "set_flag", Value: ""},
		},
	}
	p.executeActions(rule, 42, nil, nil)

	mock.mu.Lock()
	if len(mock.setFlagsCalls) != 0 {
		t.Fatal("expected no SetFlags calls for empty flag value")
	}
	mock.mu.Unlock()
}

func TestProcessMessageMatchesCondition(t *testing.T) {
	rulesRepo, contactsRepo := openPollerTestDB(t)
	rule := &db.Rule{
		Name:    "invoice-rule",
		Enabled: true,
		Groups: []db.ConditionGroup{
			{Operator: "AND", Conditions: []db.Condition{
				{Field: "subject", Operator: "contains", Value: "invoice"},
			}},
		},
		Actions: []db.Action{{Type: "move_to_folder", Value: "Finance"}},
	}
	if err := rulesRepo.Create(rule); err != nil {
		t.Fatalf("Create: %v", err)
	}

	mock := &trackedMock{
		searchUIDs: []goimap.UID{1},
		messages: map[uint32]*imap.Message{
			1: {UID: 1, Subject: "Q3 Invoice #1234"},
		},
	}
	collector := contacts.NewCollector(contactsRepo, mock)
	p := NewPoller(mock, rulesRepo, collector, nil, nil, nil, nil, nil, 50, 60, "INBOX", "", nil)

	if err := p.process(); err != nil {
		t.Fatalf("process: %v", err)
	}

	mock.mu.Lock()
	if len(mock.moveCalls) != 1 || mock.moveCalls[0].Dest != "Finance" {
		t.Fatalf("expected move to Finance, got %v", mock.moveCalls)
	}
	mock.mu.Unlock()
}

func TestProcessMessageDoesNotMatchCondition(t *testing.T) {
	rulesRepo, contactsRepo := openPollerTestDB(t)
	rule := &db.Rule{
		Name:    "invoice-rule",
		Enabled: true,
		Groups: []db.ConditionGroup{
			{Operator: "AND", Conditions: []db.Condition{
				{Field: "subject", Operator: "contains", Value: "invoice"},
			}},
		},
		Actions: []db.Action{{Type: "move_to_folder", Value: "Finance"}},
	}
	if err := rulesRepo.Create(rule); err != nil {
		t.Fatalf("Create: %v", err)
	}

	mock := &trackedMock{
		searchUIDs: []goimap.UID{1},
		messages: map[uint32]*imap.Message{
			1: {UID: 1, Subject: "Lunch plans"},
		},
	}
	collector := contacts.NewCollector(contactsRepo, mock)
	p := NewPoller(mock, rulesRepo, collector, nil, nil, nil, nil, nil, 50, 60, "INBOX", "", nil)

	if err := p.process(); err != nil {
		t.Fatalf("process: %v", err)
	}

	mock.mu.Lock()
	if len(mock.moveCalls) != 0 {
		t.Fatal("expected no move calls for non-matching message")
	}
	mock.mu.Unlock()
}

func openPollerTestDBWithLog(t *testing.T) (*db.RulesRepo, *db.LogRepo) {
	t.Helper()
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := db.Migrate(database); err != nil {
		database.Close()
		t.Fatalf("Migrate: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return db.NewRulesRepo(database), db.NewLogRepo(database)
}

func TestProcessLogsActions(t *testing.T) {
	rulesRepo, logRepo := openPollerTestDBWithLog(t)
	rule := &db.Rule{
		Name:    "log-rule",
		Enabled: true,
		Actions: []db.Action{{Type: "mark_as_read"}},
	}
	if err := rulesRepo.Create(rule); err != nil {
		t.Fatalf("Create: %v", err)
	}

	mock := &trackedMock{
		searchUIDs: []goimap.UID{1},
		messages: map[uint32]*imap.Message{
			1: {UID: 1, Subject: "Test Subject", From: []imap.Address{{Email: "sender@test.com"}}},
		},
	}
	p := NewPoller(mock, rulesRepo, nil, logRepo, nil, nil, nil, nil, 50, 60, "INBOX", "", nil)

	if err := p.process(); err != nil {
		t.Fatalf("process: %v", err)
	}

	entries, err := logRepo.ListRecent(10)
	if err != nil {
		t.Fatalf("ListRecent: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}
	e := entries[0]
	if e.Subject != "Test Subject" {
		t.Errorf("subject = %q, want 'Test Subject'", e.Subject)
	}
	if e.FromAddr != "sender@test.com" {
		t.Errorf("from = %q, want 'sender@test.com'", e.FromAddr)
	}
	if e.RuleName != "log-rule" {
		t.Errorf("rule = %q, want 'log-rule'", e.RuleName)
	}
	if e.ActionType != "mark_as_read" || e.Status != "success" {
		t.Errorf("action = %q, status = %q", e.ActionType, e.Status)
	}
}

func TestExecuteActionsDeclaredOrder(t *testing.T) {
	mock := &trackedMock{}
	p := &Poller{imapClient: mock}

	rule := &db.Rule{
		Name: "test",
		Actions: []db.Action{
			{Type: "mark_as_read"},
			{Type: "move_to_folder", Value: "Archive"},
		},
	}
	p.executeActions(rule, 10, nil, nil)

	mock.mu.Lock()
	if len(mock.moveCalls) != 1 || mock.moveCalls[0].UID != 10 {
		t.Fatalf("expected MoveMessage on uid 10, got %v", mock.moveCalls)
	}
	if len(mock.setFlagsCalls) != 1 || mock.setFlagsCalls[0].UID != 10 {
		t.Fatalf("expected SetFlags on source uid 10 (before move), got %v", mock.setFlagsCalls)
	}
	mock.mu.Unlock()
}

func TestExecuteActionsFlagAfterMoveOnDestUID(t *testing.T) {
	mock := &trackedMock{}
	p := &Poller{imapClient: mock}

	rule := &db.Rule{
		Name: "test",
		Actions: []db.Action{
			{Type: "move_to_folder", Value: "Archive"},
			{Type: "mark_as_read"},
		},
	}
	p.executeActions(rule, 10, nil, nil)

	mock.mu.Lock()
	if len(mock.moveCalls) != 1 || mock.moveCalls[0].UID != 10 {
		t.Fatalf("expected MoveMessage on uid 10, got %v", mock.moveCalls)
	}
	if len(mock.setFlagsCalls) != 1 || mock.setFlagsCalls[0].UID != 110 {
		t.Fatalf("expected SetFlags on dest uid 110 (after move), got %v", mock.setFlagsCalls)
	}
	mock.mu.Unlock()
}

func TestCheckBackupDisabled(t *testing.T) {
	p := &Poller{}
	p.checkBackup()
}

func TestCheckBackupNilSettings(t *testing.T) {
	p := &Poller{settingsRepo: nil}
	p.checkBackup()
}

func TestCheckBackupRecentlyRun(t *testing.T) {
	database := db.NewTestDB(t)
	settingsRepo := db.NewSettingsRepo(database)
	settingsRepo.Set("backup_enabled", "true")
	settingsRepo.Set("backup_frequency", "daily")
	settingsRepo.Set("last_backup", time.Now().Format(time.RFC3339))

	p := &Poller{settingsRepo: settingsRepo}
	p.checkBackup()
}

func TestCheckBackupNotEnabled(t *testing.T) {
	database := db.NewTestDB(t)
	settingsRepo := db.NewSettingsRepo(database)
	settingsRepo.Set("backup_enabled", "false")
	settingsRepo.Set("backup_frequency", "daily")
	settingsRepo.Set("last_backup", time.Now().Add(-48*time.Hour).Format(time.RFC3339))

	p := &Poller{settingsRepo: settingsRepo}
	p.checkBackup()
}

func TestUnmatchedDoesNotBlockMatchedMessages(t *testing.T) {
	rulesRepo, contactsRepo := openPollerTestDB(t)
	rule := &db.Rule{
		Name:    "match-rule",
		Enabled: true,
		Groups: []db.ConditionGroup{
			{Operator: "AND", Conditions: []db.Condition{
				{Field: "subject", Operator: "contains", Value: "match"},
			}},
		},
		Actions: []db.Action{{Type: "move_to_folder", Value: "Archive"}},
	}
	if err := rulesRepo.Create(rule); err != nil {
		t.Fatalf("Create: %v", err)
	}

	mock := &trackedMock{
		searchUIDs: []goimap.UID{10, 20, 30, 40},
		messages: map[uint32]*imap.Message{
			10: {UID: 10, Subject: "spam message"},
			20: {UID: 20, Subject: "match me"},
			30: {UID: 30, Subject: "another spam"},
			40: {UID: 40, Subject: "match me too"},
		},
	}
	collector := contacts.NewCollector(contactsRepo, mock)
	p := NewPoller(mock, rulesRepo, collector, nil, nil, nil, nil, nil, 50, 60, "INBOX", "", nil)

	if err := p.process(); err != nil {
		t.Fatalf("process: %v", err)
	}

	mock.mu.Lock()
	if len(mock.moveCalls) != 2 {
		t.Fatalf("expected 2 move calls, got %d: %v", len(mock.moveCalls), mock.moveCalls)
	}
	if mock.moveCalls[0].UID != 20 || mock.moveCalls[0].Dest != "Archive" {
		t.Errorf("first move: uid=%d dest=%s, want uid=20 dest=Archive", mock.moveCalls[0].UID, mock.moveCalls[0].Dest)
	}
	if mock.moveCalls[1].UID != 40 || mock.moveCalls[1].Dest != "Archive" {
		t.Errorf("second move: uid=%d dest=%s, want uid=40 dest=Archive", mock.moveCalls[1].UID, mock.moveCalls[1].Dest)
	}
	if len(mock.searchFolders) < 4 {
		t.Fatalf("expected at least 4 search calls (one per UID + empty check), got %d", len(mock.searchFolders))
	}
	mock.mu.Unlock()
}

func TestExecuteActionsDelete(t *testing.T) {
	mock := &trackedMock{
		folders: []imap.Folder{
			{Name: "Trash", Flags: "\\Trash"},
		},
	}
	p := &Poller{imapClient: mock}

	rule := &db.Rule{
		Name: "test",
		Actions: []db.Action{
			{Type: "delete"},
		},
	}
	p.executeActions(rule, 42, nil, nil)

	mock.mu.Lock()
	if len(mock.moveCalls) != 1 {
		t.Fatalf("expected 1 move call, got %d", len(mock.moveCalls))
	}
	if mock.moveCalls[0].UID != 42 {
		t.Fatalf("expected move uid 42, got %d", mock.moveCalls[0].UID)
	}
	if mock.moveCalls[0].Dest != "Trash" {
		t.Fatalf("expected dest Trash, got %s", mock.moveCalls[0].Dest)
	}
	mock.mu.Unlock()
}

func TestAutoReplyOncePerSenderPerDay(t *testing.T) {
	database := db.NewTestDB(t)
	logRepo := db.NewLogRepo(database)
	var sent []string
	p := &Poller{
		imapClient:    &trackedMock{},
		logRepo:       logRepo,
		cfg:           &config.Config{},
		imapEmail:     "me@icloud.com",
		autoReplyRepo: db.NewAutoReplyRepo(database),
		sendMail: func(to, from, password, subject, body string, attachments ...smtp.Attachment) error {
			sent = append(sent, to)
			return nil
		},
	}
	rule := &db.Rule{Name: "ar", Actions: []db.Action{{Type: "auto_reply", Value: "Away"}}}
	msg := &imap.Message{From: []imap.Address{{Email: "sender@test.com"}}}

	p.executeActions(rule, 1, msg, nil)
	p.executeActions(rule, 2, msg, nil)

	if len(sent) != 1 {
		t.Fatalf("expected exactly 1 auto-reply, got %d", len(sent))
	}
	entries, _ := logRepo.ListRecent(10)
	var skipped int
	for _, e := range entries {
		if e.ActionType == "auto_reply" && e.Status == "skipped" {
			skipped++
		}
	}
	if skipped != 1 {
		t.Fatalf("expected 1 skipped auto_reply log entry, got %d", skipped)
	}
}

func TestAutoReplySkipsSelf(t *testing.T) {
	database := db.NewTestDB(t)
	var sent []string
	p := &Poller{
		imapClient:    &trackedMock{},
		logRepo:       db.NewLogRepo(database),
		cfg:           &config.Config{},
		imapEmail:     "me@icloud.com",
		autoReplyRepo: db.NewAutoReplyRepo(database),
		sendMail: func(to, from, password, subject, body string, attachments ...smtp.Attachment) error {
			sent = append(sent, to)
			return nil
		},
	}
	rule := &db.Rule{Name: "ar", Actions: []db.Action{{Type: "auto_reply", Value: "Away"}}}
	msg := &imap.Message{From: []imap.Address{{Email: "me@icloud.com"}}}

	p.executeActions(rule, 1, msg, nil)

	if len(sent) != 0 {
		t.Fatalf("expected no auto-reply to self, got %d", len(sent))
	}
}

func TestAutoReplyTemplateVariables(t *testing.T) {
	database := db.NewTestDB(t)
	var lastBody string
	p := &Poller{
		imapClient:    &trackedMock{},
		logRepo:       db.NewLogRepo(database),
		cfg:           &config.Config{},
		imapEmail:     "me@icloud.com",
		autoReplyRepo: db.NewAutoReplyRepo(database),
		sendMail: func(to, from, password, subject, body string, attachments ...smtp.Attachment) error {
			lastBody = body
			return nil
		},
	}
	rule := &db.Rule{
		Name: "auto-reply-rule",
		Actions: []db.Action{{
			Type:  "auto_reply",
			Value: "You sent: [subject]. Your order [capture:order] from [from] to [to] cc [cc] via [rule_name] on [date]",
		}},
	}
	msg := &imap.Message{
		Subject: "Order #12345",
		From:    []imap.Address{{Email: "sender@test.com"}},
		To:      []imap.Address{{Email: "me@icloud.com"}},
		Cc:      []imap.Address{{Email: "cc@test.com"}},
	}
	captures := map[string]string{
		"capture:order": "12345",
		"capture:0":     "Order #12345",
	}
	p.executeActions(rule, 1, msg, captures)

	if !strings.Contains(lastBody, "You sent: Order #12345") {
		t.Errorf("expected subject expansion, got: %s", lastBody)
	}
	if !strings.Contains(lastBody, "12345") {
		t.Errorf("expected order capture, got: %s", lastBody)
	}
	if !strings.Contains(lastBody, "from sender@test.com") {
		t.Errorf("expected from expansion, got: %s", lastBody)
	}
	if !strings.Contains(lastBody, "to me@icloud.com") {
		t.Errorf("expected to expansion, got: %s", lastBody)
	}
	if !strings.Contains(lastBody, "cc cc@test.com") {
		t.Errorf("expected cc expansion, got: %s", lastBody)
	}
	if !strings.Contains(lastBody, "via auto-reply-rule") {
		t.Errorf("expected rule_name expansion, got: %s", lastBody)
	}
}

func TestExecuteActionsWebhook(t *testing.T) {
	database := db.NewTestDB(t)
	var calledURL string
	var calledBody []byte
	p := &Poller{
		imapClient:   &trackedMock{},
		logRepo:      db.NewLogRepo(database),
		statsRepo:    db.NewStatsRepo(database),
		settingsRepo: db.NewSettingsRepo(database),
		cfg:          &config.Config{},
		sendWebhook: func(url string, payload []byte, secret string) error {
			calledURL = url
			calledBody = payload
			return nil
		},
	}
	rule := &db.Rule{Name: "webhook-rule", Actions: []db.Action{{Type: "webhook", Value: "https://example.com/hook"}}}
	msg := &imap.Message{
		From:    []imap.Address{{Email: "test@example.com"}},
		Subject: "test",
		Date:    time.Now(),
		To:      []imap.Address{{Email: "rcpt@test.com"}},
		Cc:      []imap.Address{{Email: "cc@test.com"}},
	}
	p.executeActions(rule, 1, msg, nil)
	if calledURL != "https://example.com/hook" {
		t.Errorf("expected https://example.com/hook, got %s", calledURL)
	}
	if !strings.Contains(string(calledBody), "webhook-rule") {
		t.Error("payload should contain rule name")
	}
	if !strings.Contains(string(calledBody), "test@example.com") {
		t.Error("payload should contain from")
	}
	if !strings.Contains(string(calledBody), "rcpt@test.com") {
		t.Error("payload should contain to")
	}
	if !strings.Contains(string(calledBody), "cc@test.com") {
		t.Error("payload should contain cc")
	}
	entries, _ := p.logRepo.ListRecent(10)
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

func TestExecuteActionsWebhookEmptyURL(t *testing.T) {
	database := db.NewTestDB(t)
	var called bool
	p := &Poller{
		imapClient:   &trackedMock{},
		logRepo:      db.NewLogRepo(database),
		statsRepo:    db.NewStatsRepo(database),
		settingsRepo: db.NewSettingsRepo(database),
		cfg:          &config.Config{},
		sendWebhook: func(url string, payload []byte, secret string) error {
			called = true
			return nil
		},
	}
	rule := &db.Rule{Name: "webhook-empty", Actions: []db.Action{{Type: "webhook", Value: ""}}}
	msg := &imap.Message{Subject: "test", Date: time.Now()}
	p.executeActions(rule, 1, msg, nil)
	if called {
		t.Error("should not call webhook for empty URL")
	}
}
