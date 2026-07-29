package web

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"

	goimap "github.com/emersion/go-imap/v2"

	"github.com/mojoaar/icloud-mailflow/internal/db"
	"github.com/mojoaar/icloud-mailflow/internal/imap"
)

type mockIMAPClient struct {
	folders     []imap.Folder
	searchUIDs  []goimap.UID
	messages    map[uint32]*imap.Message
}

func (m *mockIMAPClient) SearchMessages(folder string, limit int) ([]goimap.UID, error) {
	return m.searchUIDs, nil
}

func (m *mockIMAPClient) FetchMessage(uid uint32) (*imap.Message, error) {
	if msg, ok := m.messages[uid]; ok {
		return msg, nil
	}
	return &imap.Message{UID: uid}, nil
}

func (m *mockIMAPClient) MoveMessage(uid uint32, dest string) (uint32, error) {
	return uid, nil
}

func (m *mockIMAPClient) SelectMailbox(name string) error {
	return nil
}

func (m *mockIMAPClient) SetFlags(uid uint32, flags []string) error {
	return nil
}

func (m *mockIMAPClient) RemoveFlags(uid uint32, flags []string) error {
	return nil
}

func (m *mockIMAPClient) CreateFolder(name string) error {
	return nil
}

func (m *mockIMAPClient) ListFolders() ([]imap.Folder, error) {
	return m.folders, nil
}

func openWebTestDB(t *testing.T) *sql.DB {
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
	return database
}

func serveHandler(h http.HandlerFunc, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}
