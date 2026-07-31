package contacts

import (
	"fmt"
	"log/slog"
	"net/mail"
	"regexp"

	"github.com/mojoaar/icloud-mailflow/internal/db"
	"github.com/mojoaar/icloud-mailflow/internal/imap"
)

var emailRE = regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)

type Collector struct {
	repo   *db.ContactsRepo
	client imap.Client
}

func NewCollector(repo *db.ContactsRepo, client imap.Client) *Collector {
	return &Collector{repo: repo, client: client}
}

func (c *Collector) CollectFromMessage(msg *imap.Message) error {
	addresses := append(msg.From, msg.To...)
	addresses = append(addresses, msg.Cc...)
	var contacts []db.Contact
	for _, addr := range addresses {
		if addr.Email == "" {
			continue
		}
		contacts = append(contacts, db.Contact{
			Email: addr.Email,
			Name:  addr.Name,
		})
	}
	if len(contacts) == 0 {
		return nil
	}
	return c.repo.UpsertBatch(contacts)
}

func (c *Collector) CollectFromBody(body string) {
	matches := emailRE.FindAllString(body, -1)
	for _, email := range matches {
		if _, err := mail.ParseAddress(email); err != nil {
			continue
		}
		c.repo.Upsert(email, "")
	}
}

func (c *Collector) SeedFromFolder(folder string) error {
	if c.client == nil {
		return fmt.Errorf("IMAP not connected")
	}
	uids, err := c.client.SearchMessages(folder, 200, 0)
	if err != nil {
		return err
	}
	for i, j := 0, len(uids)-1; i < j; i, j = i+1, j-1 {
		uids[i], uids[j] = uids[j], uids[i]
	}
	slog.Debug("seeding contacts from folder", "folder", folder, "count", len(uids))
	for _, uid := range uids {
		msg, err := c.client.FetchMessage(uint32(uid))
		if err != nil {
			slog.Warn("failed to fetch message during seed", "uid", uid, "error", err)
			continue
		}
		c.CollectFromMessage(msg)
	}
	return nil
}
