package contacts

import (
	"errors"
	"testing"

	goimap "github.com/emersion/go-imap/v2"

	"github.com/mojoaar/icloud-mailflow/internal/db"
	"github.com/mojoaar/icloud-mailflow/internal/imap"
)

type mockClient struct {
	searchUIDs []goimap.UID
	messages   map[uint32]*imap.Message
	searchErr  error
	fetchErr   error
}

func (m *mockClient) SearchMessages(folder string, limit int, minUID uint32) ([]goimap.UID, error) {
	return m.searchUIDs, m.searchErr
}

func (m *mockClient) FetchMessage(uid uint32) (*imap.Message, error) {
	if m.fetchErr != nil {
		return nil, m.fetchErr
	}
	if msg, ok := m.messages[uid]; ok {
		return msg, nil
	}
	return &imap.Message{UID: uid}, nil
}

func (m *mockClient) FetchMessages(uids []goimap.UID) ([]*imap.Message, error) {
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

func (m *mockClient) MoveMessage(uid uint32, dest string) (uint32, error) { return uid, nil }
func (m *mockClient) SelectMailbox(name string) error        { return nil }
func (m *mockClient) SetFlags(uid uint32, flags []string) error  { return nil }
func (m *mockClient) RemoveFlags(uid uint32, flags []string) error { return nil }
func (m *mockClient) CreateFolder(name string) error             { return nil }
func (m *mockClient) ListFolders() ([]imap.Folder, error)       { return nil, nil }
func (m *mockClient) FetchMessageHeader(uid uint32, headerName string) (string, error) { return "", nil }
func (m *mockClient) FetchMessageBody(uid uint32) (string, error)                      { return "", nil }
func (m *mockClient) FetchRawMessage(uid uint32) ([]byte, error)                       { return nil, nil }

func openContactsTestDB(t *testing.T) *db.ContactsRepo {
	t.Helper()
	return db.NewContactsRepo(db.NewTestDB(t))
}

func TestCollectFromMessage(t *testing.T) {
	repo := openContactsTestDB(t)
	c := NewCollector(repo, nil)

	msg := &imap.Message{
		From: []imap.Address{{Name: "Alice", Email: "alice@example.com"}},
		To:   []imap.Address{{Name: "Bob", Email: "bob@example.com"}},
		Cc:   []imap.Address{{Name: "Carol", Email: "carol@example.com"}},
	}

	if err := c.CollectFromMessage(msg); err != nil {
		t.Fatalf("CollectFromMessage: %v", err)
	}

	contacts, err := repo.Search("example.com")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(contacts) != 3 {
		t.Fatalf("expected 3 contacts, got %d", len(contacts))
	}
}

func TestCollectFromMessageEmptyAddresses(t *testing.T) {
	repo := openContactsTestDB(t)
	c := NewCollector(repo, nil)

	msg := &imap.Message{
		From: []imap.Address{},
		To:   []imap.Address{},
		Cc:   []imap.Address{},
	}

	if err := c.CollectFromMessage(msg); err != nil {
		t.Fatalf("CollectFromMessage: %v", err)
	}

	contacts, _ := repo.Search("example.com")
	if len(contacts) != 0 {
		t.Fatalf("expected 0 contacts, got %d", len(contacts))
	}
}

func TestCollectFromMessageSkipsEmptyEmails(t *testing.T) {
	repo := openContactsTestDB(t)
	c := NewCollector(repo, nil)

	msg := &imap.Message{
		From: []imap.Address{{Name: "No Email", Email: ""}},
		To:   []imap.Address{{Name: "Valid", Email: "valid@example.com"}},
	}

	if err := c.CollectFromMessage(msg); err != nil {
		t.Fatalf("CollectFromMessage: %v", err)
	}

	contacts, _ := repo.Search("example.com")
	if len(contacts) != 1 {
		t.Fatalf("expected 1 contact, got %d", len(contacts))
	}
	if contacts[0].Email != "valid@example.com" {
		t.Fatalf("expected valid@example.com, got %s", contacts[0].Email)
	}
}

func TestCollectFromBody(t *testing.T) {
	repo := openContactsTestDB(t)
	c := NewCollector(repo, nil)

	c.CollectFromBody("Contact me at foo@bar.com or baz@qux.org")

	contacts, err := repo.Search("foo@bar.com")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(contacts) != 1 {
		t.Fatalf("expected 1 contact for foo@bar.com, got %d", len(contacts))
	}
	if contacts[0].Email != "foo@bar.com" {
		t.Fatalf("expected foo@bar.com, got %s", contacts[0].Email)
	}
}

func TestCollectFromBodyNoMatches(t *testing.T) {
	repo := openContactsTestDB(t)
	c := NewCollector(repo, nil)

	c.CollectFromBody("Hello world, no emails here")

	contacts, _ := repo.Search("hello")
	if len(contacts) != 0 {
		t.Fatalf("expected 0 contacts, got %d", len(contacts))
	}
}

func TestSeedFromFolder(t *testing.T) {
	repo := openContactsTestDB(t)
	mock := &mockClient{
		searchUIDs: []goimap.UID{1, 2},
		messages: map[uint32]*imap.Message{
			1: {From: []imap.Address{{Email: "a@x.com"}}},
			2: {From: []imap.Address{{Email: "b@x.com"}}},
		},
	}
	c := NewCollector(repo, mock)

	if err := c.SeedFromFolder("INBOX"); err != nil {
		t.Fatalf("SeedFromFolder: %v", err)
	}

	contacts, _ := repo.Search("x.com")
	if len(contacts) != 2 {
		t.Fatalf("expected 2 contacts, got %d", len(contacts))
	}
}

func TestSeedFromFolderSearchError(t *testing.T) {
	repo := openContactsTestDB(t)
	mock := &mockClient{searchErr: errors.New("search error")}
	c := NewCollector(repo, mock)

	err := c.SeedFromFolder("INBOX")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestSeedFromFolderEmptyFolder(t *testing.T) {
	repo := openContactsTestDB(t)
	mock := &mockClient{searchUIDs: []goimap.UID{}}
	c := NewCollector(repo, mock)

	if err := c.SeedFromFolder("INBOX"); err != nil {
		t.Fatalf("SeedFromFolder: %v", err)
	}

	contacts, _ := repo.Search("x.com")
	if len(contacts) != 0 {
		t.Fatalf("expected 0 contacts, got %d", len(contacts))
	}
}
