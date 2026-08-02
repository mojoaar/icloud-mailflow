package web

import (
	"net/http"

	"github.com/mojoaar/icloud-mailflow/internal/db"
	"github.com/mojoaar/icloud-mailflow/internal/imap"
)

func contactsSearchHandler(repo *db.ContactsRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		contacts, err := repo.Search(q)
		if err != nil {
			contacts = []db.Contact{}
		}
		renderPartial(w, "contacts_list", map[string]any{"Contacts": contacts})
	}
}

func foldersListHandler(imapClient imap.Client, repo *db.FoldersRepo, settingsRepo *db.SettingsRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if imapClient != nil {
			if imapFolders, err := imapClient.ListFolders(); err == nil {
				syncFoldersToDB(imapFolders, repo)
			}
		}
		folders, _ := repo.List()
		source, _ := settingsRepo.Get("source_folder")
		if source != "" {
			found := false
			for _, f := range folders {
				if f.Name == source {
					found = true
					break
				}
			}
			if !found {
				w.Header().Set("HX-Trigger", `{"showToast":{"type":"error","message":"Source folder '`+source+`' not found on IMAP server"}}`)
			}
		}
		renderPartial(w, "folders_list", map[string]any{"Folders": folders})
	}
}
