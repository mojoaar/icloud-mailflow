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

func foldersListHandler(imapClient *imap.IMAPClient, repo *db.FoldersRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if imapClient != nil {
			imapFolders, err := imapClient.ListFolders()
			if err == nil {
				var dbFolders []db.Folder
				for _, f := range imapFolders {
					dbFolders = append(dbFolders, db.Folder{
						Name:  f.Name,
						Path:  f.Path,
						Flags: f.Flags,
					})
				}
				repo.Sync(dbFolders)
			}
		}
		folders, _ := repo.List()
		renderPartial(w, "folders_list", map[string]any{"Folders": folders})
	}
}
