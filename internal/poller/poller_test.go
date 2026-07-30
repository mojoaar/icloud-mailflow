package poller

import (
	"errors"
	"sync"
	"testing"
	"time"

	goimap "github.com/emersion/go-imap/v2"

	"github.com/mojoaar/icloud-mailflow/internal/contacts"
	"github.com/mojoaar/icloud-mailflow/internal/db"
	"github.com/mojoaar/icloud-mailflow/internal/imap"
)

type trackedMock struct {
	mu              sync.Mutex
	searchUIDs      []goimap.UID
	messages        map[uint32]*imap.Message
	searchErr       error
	fetchErr        error
	moveCalls       []moveCall
	setFlagsCalls   []setFlagsCall
	removeFlagsCalls []removeFlagsCall
	searchFolders   []string
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

func (m *trackedMock) SearchMessages(folder string, limit int) ([]goimap.UID, error) {
	m.mu.Lock()
	m.searchFolders = append(m.searchFolders, folder)
	m.mu.Unlock()
	return m.searchUIDs, m.searchErr
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

func (m *trackedMock) ListFolders() ([]imap.Folder, error) { return nil, nil }

func openPollerTestDB(t *testing.T) (*db.RulesRepo, *db.ContactsRepo) {
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
	p := NewPoller(mock, rulesRepo, collector, nil, 50, 60, "INBOX")

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
	p := NewPoller(mock, rulesRepo, collector, nil, 50, 60, "INBOX")

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
	p := NewPoller(mock, rulesRepo, collector, nil, 50, 60, "INBOX")

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
	p := NewPoller(mock, rulesRepo, collector, nil, 50, 60, "INBOX")

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
	p := NewPoller(mock, rulesRepo, collector, nil, 50, 60, "INBOX")

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
	p := NewPoller(mock, rulesRepo, nil, nil, 50, 60, "INBOX")

	if err := p.process(); err != nil {
		t.Fatalf("process with nil collector: %v", err)
	}
}

func TestProcessSearchesSourceFolder(t *testing.T) {
	rulesRepo, contactsRepo := openPollerTestDB(t)
	mock := &trackedMock{searchUIDs: []goimap.UID{}}
	collector := contacts.NewCollector(contactsRepo, mock)
	p := NewPoller(mock, rulesRepo, collector, nil, 50, 60, "Archive")

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
	p.executeActions(rule, 42, nil)

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
	p.executeActions(rule, 42, nil)

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
	p := NewPoller(mock, rulesRepo, collector, nil, 50, 1, "INBOX")

	p.Start()
	time.Sleep(50 * time.Millisecond)
	p.Stop()
}

func TestDoubleStart(t *testing.T) {
	rulesRepo, contactsRepo := openPollerTestDB(t)
	mock := &trackedMock{searchUIDs: []goimap.UID{}}
	collector := contacts.NewCollector(contactsRepo, mock)
	p := NewPoller(mock, rulesRepo, collector, nil, 50, 1, "INBOX")

	p.Start()
	p.Start()
	p.Stop()
}

func TestDoubleStop(t *testing.T) {
	rulesRepo, contactsRepo := openPollerTestDB(t)
	mock := &trackedMock{searchUIDs: []goimap.UID{}}
	collector := contacts.NewCollector(contactsRepo, mock)
	p := NewPoller(mock, rulesRepo, collector, nil, 50, 1, "INBOX")

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
	p.executeActions(rule, 42, nil)

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
	p.executeActions(rule, 7, nil)

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
	p.executeActions(rule, 10, nil)

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
	p.executeActions(rule, 42, nil)

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
	p.executeActions(rule, 42, nil)

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
	p := NewPoller(mock, rulesRepo, collector, nil, 50, 60, "INBOX")

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
	p := NewPoller(mock, rulesRepo, collector, nil, 50, 60, "INBOX")

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
	p := NewPoller(mock, rulesRepo, nil, logRepo, 50, 60, "INBOX")

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

func TestExecuteActionsFlagOnDestUID(t *testing.T) {
	mock := &trackedMock{}
	p := &Poller{imapClient: mock}

	rule := &db.Rule{
		Name: "test",
		Actions: []db.Action{
			{Type: "mark_as_read"},
			{Type: "move_to_folder", Value: "Archive"},
		},
	}
	p.executeActions(rule, 10, nil)

	mock.mu.Lock()
	if len(mock.moveCalls) != 1 || mock.moveCalls[0].UID != 10 {
		t.Fatalf("expected MoveMessage on uid 10, got %v", mock.moveCalls)
	}
	if len(mock.setFlagsCalls) != 1 || mock.setFlagsCalls[0].UID != 110 {
		t.Fatalf("expected SetFlags on dest uid 110, got %v", mock.setFlagsCalls)
	}
	mock.mu.Unlock()
}
