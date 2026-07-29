package imap

import (
	"testing"
	"time"

	goimap "github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
)

func makeAddr(name, mailbox, host string) goimap.Address {
	return goimap.Address{Name: name, Mailbox: mailbox, Host: host}
}

func TestConvertAddresses(t *testing.T) {
	addrs := []goimap.Address{
		makeAddr("Alice", "alice", "example.com"),
		makeAddr("", "bob", "example.com"),
	}

	result := convertAddresses(addrs)

	if len(result) != 2 {
		t.Fatalf("len = %d, want 2", len(result))
	}
	if result[0].Name != "Alice" {
		t.Errorf("Name[0] = %q, want Alice", result[0].Name)
	}
	if result[0].Email != "alice@example.com" {
		t.Errorf("Email[0] = %q, want alice@example.com", result[0].Email)
	}
	if result[1].Name != "" {
		t.Errorf("Name[1] = %q, want empty", result[1].Name)
	}
	if result[1].Email != "bob@example.com" {
		t.Errorf("Email[1] = %q, want bob@example.com", result[1].Email)
	}
}

func TestConvertAddressesEmpty(t *testing.T) {
	result := convertAddresses([]goimap.Address{})
	if len(result) != 0 {
		t.Errorf("len = %d, want 0", len(result))
	}
}

func TestConvertMessageFull(t *testing.T) {
	buf := &imapclient.FetchMessageBuffer{
		UID: 42,
		Flags: []goimap.Flag{
			goimap.FlagSeen,
			goimap.FlagAnswered,
		},
		Envelope: &goimap.Envelope{
			Date:    time.Now(),
			Subject: "Hello World",
			From:    []goimap.Address{makeAddr("Alice", "alice", "example.com")},
			To:      []goimap.Address{makeAddr("Bob", "bob", "example.com")},
			Cc:      []goimap.Address{makeAddr("Carol", "carol", "example.com")},
		},
		BodyStructure: &goimap.BodyStructureSinglePart{
			Type:    "text",
			Subtype: "plain",
			Extended: &goimap.BodyStructureSinglePartExt{
				Disposition: &goimap.BodyStructureDisposition{
					Value: "inline",
				},
			},
		},
	}

	msg := convertMessage(buf)

	if msg.UID != 42 {
		t.Errorf("UID = %d, want 42", msg.UID)
	}
	if msg.Subject != "Hello World" {
		t.Errorf("Subject = %q, want Hello World", msg.Subject)
	}
	if len(msg.From) != 1 || msg.From[0].Email != "alice@example.com" {
		t.Errorf("From = %v", msg.From)
	}
	if len(msg.To) != 1 || msg.To[0].Email != "bob@example.com" {
		t.Errorf("To = %v", msg.To)
	}
	if len(msg.Cc) != 1 || msg.Cc[0].Email != "carol@example.com" {
		t.Errorf("Cc = %v", msg.Cc)
	}
	if msg.HasAttach {
		t.Error("HasAttach should be false for inline disposition")
	}
	if len(msg.Flags) != 2 {
		t.Errorf("Flags len = %d, want 2", len(msg.Flags))
	}
}

func TestConvertMessageMinimal(t *testing.T) {
	buf := &imapclient.FetchMessageBuffer{
		UID: 1,
	}

	msg := convertMessage(buf)

	if msg.UID != 1 {
		t.Errorf("UID = %d, want 1", msg.UID)
	}
	if msg.Subject != "" {
		t.Errorf("Subject = %q, want empty", msg.Subject)
	}
	if len(msg.From) != 0 {
		t.Errorf("From len = %d, want 0", len(msg.From))
	}
	if len(msg.To) != 0 {
		t.Errorf("To len = %d, want 0", len(msg.To))
	}
	if len(msg.Cc) != 0 {
		t.Errorf("Cc len = %d, want 0", len(msg.Cc))
	}
	if msg.HasAttach {
		t.Error("HasAttach should be false")
	}
	if len(msg.Flags) != 0 {
		t.Errorf("Flags len = %d, want 0", len(msg.Flags))
	}
}

func TestHasAttachmentSinglePartInline(t *testing.T) {
	buf := &imapclient.FetchMessageBuffer{
		BodyStructure: &goimap.BodyStructureSinglePart{
			Type:    "text",
			Subtype: "plain",
			Extended: &goimap.BodyStructureSinglePartExt{
				Disposition: &goimap.BodyStructureDisposition{
					Value: "inline",
				},
			},
		},
	}
	if hasAttachment(buf) {
		t.Error("inline disposition should not be attachment")
	}
}

func TestHasAttachmentSinglePart(t *testing.T) {
	buf := &imapclient.FetchMessageBuffer{
		BodyStructure: &goimap.BodyStructureSinglePart{
			Type:    "application",
			Subtype: "pdf",
			Extended: &goimap.BodyStructureSinglePartExt{
				Disposition: &goimap.BodyStructureDisposition{
					Value: "attachment",
				},
			},
		},
	}
	if !hasAttachment(buf) {
		t.Error("should detect attachment")
	}
}

func TestHasAttachmentMultiPartWithNested(t *testing.T) {
	buf := &imapclient.FetchMessageBuffer{
		BodyStructure: &goimap.BodyStructureMultiPart{
			Subtype: "mixed",
			Children: []goimap.BodyStructure{
				&goimap.BodyStructureSinglePart{
					Type:    "text",
					Subtype: "plain",
				},
				&goimap.BodyStructureSinglePart{
					Type:    "application",
					Subtype: "pdf",
					Extended: &goimap.BodyStructureSinglePartExt{
						Disposition: &goimap.BodyStructureDisposition{
							Value: "attachment",
						},
					},
				},
			},
		},
	}
	if !hasAttachment(buf) {
		t.Error("should detect attachment in multipart")
	}
}

func TestHasAttachmentNoBodyStructure(t *testing.T) {
	buf := &imapclient.FetchMessageBuffer{}
	if hasAttachment(buf) {
		t.Error("no body structure should mean no attachment")
	}
}

func TestHasAttachmentTextPlainNoDisp(t *testing.T) {
	buf := &imapclient.FetchMessageBuffer{
		BodyStructure: &goimap.BodyStructureSinglePart{
			Type:    "text",
			Subtype: "plain",
		},
	}
	if hasAttachment(buf) {
		t.Error("text/plain without disposition should not be attachment")
	}
}

func TestHasAttachmentMultiPartImageInline(t *testing.T) {
	buf := &imapclient.FetchMessageBuffer{
		BodyStructure: &goimap.BodyStructureMultiPart{
			Subtype: "related",
			Children: []goimap.BodyStructure{
				&goimap.BodyStructureSinglePart{
					Type:    "text",
					Subtype: "html",
				},
				&goimap.BodyStructureSinglePart{
					Type:    "image",
					Subtype: "png",
					Extended: &goimap.BodyStructureSinglePartExt{
						Disposition: &goimap.BodyStructureDisposition{
							Value: "inline",
						},
					},
				},
			},
		},
	}
	if hasAttachment(buf) {
		t.Error("only inline parts should not count as attachments")
	}
}
