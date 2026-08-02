package web

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	goimap "github.com/emersion/go-imap/v2"

	"github.com/mojoaar/icloud-mailflow/internal/db"
	"github.com/mojoaar/icloud-mailflow/internal/imap"
)

type mockIMAPClient struct {
	folders    []imap.Folder
	searchUIDs []goimap.UID
	messages   map[uint32]*imap.Message
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

func (m *mockIMAPClient) FetchMessageHeader(uid uint32, headerName string) (string, error) {
	return "", nil
}

func (m *mockIMAPClient) FetchMessageBody(uid uint32) (string, error) {
	return "", nil
}

func (m *mockIMAPClient) FetchRawMessage(uid uint32) ([]byte, error) {
	return nil, nil
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

func TestClientIPStripsPort(t *testing.T) {
	cases := map[string]string{
		"1.2.3.4:5678": "1.2.3.4",
		"1.2.3.4":      "1.2.3.4",
		"[::1]:80":     "::1",
	}
	for remote, want := range cases {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = remote
		if got := clientIP(r); got != want {
			t.Errorf("clientIP(%q) = %q, want %q", remote, got, want)
		}
	}
}

func TestLoginRateLimitIgnoresPort(t *testing.T) {
	limiter := newRateLimiter()
	for i := 0; i < 5; i++ {
		if !limiter.allow("9.9.9.9", 5, time.Minute) {
			t.Fatalf("attempt %d should be allowed", i+1)
		}
	}
	if limiter.allow("9.9.9.9", 5, time.Minute) {
		t.Fatal("6th attempt from same host must be limited")
	}
}
