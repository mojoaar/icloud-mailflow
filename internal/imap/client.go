package imap

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"strings"

	goimap "github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"

	"github.com/mojoaar/icloud-mailflow/internal/config"
)

type Address struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type Folder struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	Flags string `json:"flags"`
}

type Message struct {
	UID       uint32    `json:"uid"`
	Subject   string    `json:"subject"`
	From      []Address `json:"from"`
	To        []Address `json:"to"`
	Cc        []Address `json:"cc"`
	HasAttach bool      `json:"has_attachment"`
	Flags     []string  `json:"flags"`
}

type Client interface {
	SearchMessages(folder string, limit int, minUID uint32) ([]goimap.UID, error)
	FetchMessage(uid uint32) (*Message, error)
	FetchMessages(uids []goimap.UID) ([]*Message, error)
	MoveMessage(uid uint32, dest string) (uint32, error)
	SelectMailbox(name string) error
	SetFlags(uid uint32, flags []string) error
	RemoveFlags(uid uint32, flags []string) error
	CreateFolder(name string) error
	ListFolders() ([]Folder, error)
}

type IMAPClient struct {
	cfg    *config.Config
	client *imapclient.Client
}

func New(cfg *config.Config) *IMAPClient {
	return &IMAPClient{cfg: cfg}
}

func (c *IMAPClient) Connect() error {
	addr := fmt.Sprintf("%s:%d", c.cfg.IMAPServer, c.cfg.IMAPPort)
	conn, err := tls.Dial("tcp", addr, &tls.Config{MinVersion: tls.VersionTLS12})
	if err != nil {
		return fmt.Errorf("tls dial: %w", err)
	}
	if c.cfg.IMAPPassword == "" {
		return fmt.Errorf("imap password is empty")
	}
	client := imapclient.New(conn, &imapclient.Options{})
	if err := client.Login(c.cfg.IMAPEmail, c.cfg.IMAPPassword).Wait(); err != nil {
		return fmt.Errorf("login: %w", err)
	}
	c.client = client
	slog.Debug("connected to imap server", "server", addr)
	return nil
}

func (c *IMAPClient) Close() error {
	if c.client == nil {
		return nil
	}
	return c.client.Close()
}

func (c *IMAPClient) ListFolders() ([]Folder, error) {
	items, err := c.client.List("", "*", nil).Collect()
	if err != nil {
		return nil, fmt.Errorf("list folders: %w", err)
	}
	var folders []Folder
	for _, item := range items {
		name := item.Mailbox
		flags := ""
		for _, a := range item.Attrs {
			flags += string(a) + " "
		}
		folders = append(folders, Folder{
			Name:  name,
			Path:  name,
			Flags: strings.TrimSpace(flags),
		})
	}
	return folders, nil
}

func (c *IMAPClient) SelectFolder(name string) (*goimap.SelectData, error) {
	return c.client.Select(name, nil).Wait()
}

func (c *IMAPClient) SearchMessages(folder string, limit int, minUID uint32) ([]goimap.UID, error) {
	if _, err := c.SelectFolder(folder); err != nil {
		return nil, fmt.Errorf("select %s: %w", folder, err)
	}
	criteria := &goimap.SearchCriteria{}
	if minUID > 0 {
		criteria.UID = []goimap.UIDSet{{{Start: goimap.UID(minUID), Stop: 0}}}
	}
	data, err := c.client.UIDSearch(criteria, &goimap.SearchOptions{}).Wait()
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	uids := data.AllUIDs()
	if limit > 0 && len(uids) > limit {
		uids = uids[:limit]
	}
	return uids, nil
}

func (c *IMAPClient) FetchMessage(uid uint32) (*Message, error) {
	msgs, err := c.FetchMessages([]goimap.UID{goimap.UID(uid)})
	if err != nil {
		return nil, err
	}
	if len(msgs) == 0 {
		return nil, fmt.Errorf("message uid %d not found", uid)
	}
	return msgs[0], nil
}

func (c *IMAPClient) FetchMessages(uids []goimap.UID) ([]*Message, error) {
	if len(uids) == 0 {
		return nil, nil
	}
	seqSet := goimap.UIDSetNum(uids...)
	opts := &goimap.FetchOptions{
		Envelope:      true,
		Flags:         true,
		BodyStructure: &goimap.FetchItemBodyStructure{},
	}
	raw, err := c.client.Fetch(seqSet, opts).Collect()
	if err != nil {
		return nil, fmt.Errorf("fetch uids: %w", err)
	}
	msgs := make([]*Message, len(raw))
	for i, m := range raw {
		msgs[i] = convertMessage(m)
	}
	return msgs, nil
}

func (c *IMAPClient) MoveMessage(uid uint32, dest string) (uint32, error) {
	seqSet := goimap.UIDSetNum(goimap.UID(uid))
	data, err := c.client.Move(seqSet, dest).Wait()
	if err != nil {
		return uid, fmt.Errorf("move uid %d to %s: %w", uid, dest, err)
	}
	if uidSet, ok := data.DestUIDs.(goimap.UIDSet); ok && len(uidSet) > 0 {
		return uint32(uidSet[0].Start), nil
	}
	return uid, nil
}

func (c *IMAPClient) SelectMailbox(name string) error {
	_, err := c.SelectFolder(name)
	return err
}

func (c *IMAPClient) CreateFolder(name string) error {
	return c.client.Create(name, nil).Wait()
}

func (c *IMAPClient) SetFlags(uid uint32, flags []string) error {
	seqSet := goimap.UIDSetNum(goimap.UID(uid))
	imapFlags := make([]goimap.Flag, len(flags))
	for i, f := range flags {
		imapFlags[i] = goimap.Flag(f)
	}
	store := &goimap.StoreFlags{
		Op:    goimap.StoreFlagsAdd,
		Flags: imapFlags,
	}
	return c.client.Store(seqSet, store, nil).Close()
}

func (c *IMAPClient) RemoveFlags(uid uint32, flags []string) error {
	seqSet := goimap.UIDSetNum(goimap.UID(uid))
	imapFlags := make([]goimap.Flag, len(flags))
	for i, f := range flags {
		imapFlags[i] = goimap.Flag(f)
	}
	store := &goimap.StoreFlags{
		Op:    goimap.StoreFlagsDel,
		Flags: imapFlags,
	}
	return c.client.Store(seqSet, store, nil).Close()
}

func convertMessage(buf *imapclient.FetchMessageBuffer) *Message {
	msg := &Message{
		UID:   uint32(buf.UID),
		Flags: make([]string, len(buf.Flags)),
	}
	for i, f := range buf.Flags {
		msg.Flags[i] = string(f)
	}
	if buf.Envelope != nil {
		msg.Subject = buf.Envelope.Subject
		msg.From = convertAddresses(buf.Envelope.From)
		msg.To = convertAddresses(buf.Envelope.To)
		msg.Cc = convertAddresses(buf.Envelope.Cc)
	}
	msg.HasAttach = hasAttachment(buf)
	return msg
}

func convertAddresses(addrs []goimap.Address) []Address {
	out := make([]Address, len(addrs))
	for i, a := range addrs {
		out[i] = Address{
			Name:  a.Name,
			Email: a.Addr(),
		}
	}
	return out
}

func hasAttachment(buf *imapclient.FetchMessageBuffer) bool {
	if buf.BodyStructure == nil {
		return false
	}
	var found bool
	buf.BodyStructure.Walk(func(path []int, bs goimap.BodyStructure) bool {
		if d := bs.Disposition(); d != nil && strings.EqualFold(d.Value, "attachment") {
			found = true
			return false
		}
		return true
	})
	return found
}
