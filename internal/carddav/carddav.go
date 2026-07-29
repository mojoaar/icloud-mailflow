package carddav

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/emersion/go-vcard"
	webcarddav "github.com/emersion/go-webdav/carddav"

	"github.com/mojoaar/icloud-mailflow/internal/db"
)

type basicAuthTransport struct {
	Username string
	Password string
	Next     http.RoundTripper
}

func (t *basicAuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.SetBasicAuth(t.Username, t.Password)
	return t.Next.RoundTrip(req)
}

type Importer struct {
	contactsRepo *db.ContactsRepo
}

func NewImporter(repo *db.ContactsRepo) *Importer {
	return &Importer{contactsRepo: repo}
}

func (i *Importer) ImportFromiCloud(email, password string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	httpClient := &http.Client{
		Transport: &basicAuthTransport{
			Username: email,
			Password: password,
			Next:     http.DefaultTransport,
		},
	}

	client, err := webcarddav.NewClient(httpClient, "https://contacts.icloud.com/")
	if err != nil {
		return 0, fmt.Errorf("carddav client: %w", err)
	}

	principal, err := client.FindCurrentUserPrincipal(ctx)
	if err != nil {
		return 0, fmt.Errorf("find principal: %w", err)
	}

	homeSet, err := client.FindAddressBookHomeSet(ctx, principal)
	if err != nil {
		return 0, fmt.Errorf("find address book home: %w", err)
	}

	books, err := client.FindAddressBooks(ctx, homeSet)
	if err != nil {
		return 0, fmt.Errorf("find address books: %w", err)
	}

	query := &webcarddav.AddressBookQuery{
		DataRequest: webcarddav.AddressDataRequest{AllProp: true},
	}

	var contacts []db.Contact
	for _, book := range books {
		objects, err := client.QueryAddressBook(ctx, book.Path, query)
		if err != nil {
			slog.Warn("failed to query address book", "book", book.Name, "path", book.Path, "error", err)
			continue
		}
		for _, obj := range objects {
			name, email := extractContact(obj.Card)
			if email == "" {
				continue
			}
			contacts = append(contacts, db.Contact{Email: email, Name: name})
		}
	}

	if len(contacts) == 0 {
		return 0, nil
	}

	return len(contacts), i.contactsRepo.UpsertBatch(contacts)
}

func extractContact(card vcard.Card) (name, email string) {
	if n := card.Name(); n != nil {
		if n.GivenName != "" || n.FamilyName != "" {
			name = n.GivenName
			if n.FamilyName != "" {
				if name != "" {
					name += " "
				}
				name += n.FamilyName
			}
		}
	}
	if name == "" {
		if fn := card["FN"]; len(fn) > 0 {
			name = fn[0].Value
		}
	}
	if name == "" {
		if emails := card[vcard.FieldEmail]; len(emails) > 0 {
			name = emails[0].Value
		}
	}

	if emails := card[vcard.FieldEmail]; len(emails) > 0 {
		email = emails[0].Value
	}

	return
}
