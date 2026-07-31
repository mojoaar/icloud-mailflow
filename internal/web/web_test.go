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

func (m *mockIMAPClient) SearchMessages(folder string, limit int, minUID uint32) ([]goimap.UID, error) {
	return m.searchUIDs, nil
}

func (m *mockIMAPClient) FetchMessage(uid uint32) (*imap.Message, error) {
	if msg, ok := m.messages[uid]; ok {
		return msg, nil
	}
	return &imap.Message{UID: uid}, nil
}

func (m *mockIMAPClient) FetchMessages(uids []goimap.UID) ([]*imap.Message, error) {
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
	return db.NewTestDB(t)
}

func serveHandler(h http.HandlerFunc, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}
