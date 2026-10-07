package smtp

import (
	"bytes"
	"errors"
	"net/smtp"
	"strings"
	"testing"
)

func TestSendSuccess(t *testing.T) {
	origSend := sendMail
	defer func() { sendMail = origSend }()

	var sentAddr string
	var sentFrom string
	var sentTo []string
	var sentMsg []byte

	sendMail = func(addr string, a smtp.Auth, from string, to []string, msg []byte) error {
		sentAddr = addr
		sentFrom = from
		sentTo = to
		sentMsg = msg
		return nil
	}

	att := Attachment{Name: "data.json", Data: []byte(`{"hello":"world"}`)}
	err := Send("recipient@example.com", "sender@example.com", "secret", "Hello Test", "This is a body", att)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if sentAddr != smtpHost {
		t.Errorf("sentAddr = %s, want %s", sentAddr, smtpHost)
	}
	if sentFrom != "sender@example.com" {
		t.Errorf("sentFrom = %s, want sender@example.com", sentFrom)
	}
	if len(sentTo) != 1 || sentTo[0] != "recipient@example.com" {
		t.Errorf("sentTo = %v, want recipient@example.com", sentTo)
	}

	msgStr := string(sentMsg)
	if !strings.Contains(msgStr, "From: sender@example.com\r\n") {
		t.Error("missing From header in MIME body")
	}
	if !strings.Contains(msgStr, "To: recipient@example.com\r\n") {
		t.Error("missing To header in MIME body")
	}
	if !strings.Contains(msgStr, "This is a body") {
		t.Error("missing body text in MIME body")
	}
	if !strings.Contains(msgStr, "filename=data.json") {
		t.Error("missing attachment filename in MIME body")
	}
	if !strings.Contains(msgStr, `{"hello":"world"}`) {
		t.Error("missing attachment payload in MIME body")
	}
}

func TestSendCRLFRejection(t *testing.T) {
	err := Send("recipient@example.com\r\nBcc: evil@example.com", "sender@example.com", "pass", "Subject", "Body")
	if err == nil || !strings.Contains(err.Error(), "CRLF not allowed") {
		t.Errorf("expected CRLF error on To, got: %v", err)
	}

	err = Send("recipient@example.com", "sender@example.com\r\nBcc: evil@example.com", "pass", "Subject", "Body")
	if err == nil || !strings.Contains(err.Error(), "CRLF not allowed") {
		t.Errorf("expected CRLF error on From, got: %v", err)
	}
}

func TestSendMailErrorPropagated(t *testing.T) {
	origSend := sendMail
	defer func() { sendMail = origSend }()

	sendMail = func(addr string, a smtp.Auth, from string, to []string, msg []byte) error {
		return errors.New("auth failure")
	}

	err := Send("recipient@example.com", "sender@example.com", "pass", "Subject", "Body")
	if err == nil || err.Error() != "auth failure" {
		t.Errorf("expected auth failure, got: %v", err)
	}
}

func TestSendRawSuccess(t *testing.T) {
	origSend := sendMail
	defer func() { sendMail = origSend }()

	var sentMsg []byte
	sendMail = func(addr string, a smtp.Auth, from string, to []string, msg []byte) error {
		sentMsg = msg
		return nil
	}

	raw := []byte("Subject: Raw Message\r\n\r\nRaw Body")
	err := SendRaw("recipient@example.com", "sender@example.com", "secret", raw)
	if err != nil {
		t.Fatalf("SendRaw failed: %v", err)
	}
	if !bytes.Equal(sentMsg, raw) {
		t.Errorf("sentMsg = %q, want %q", sentMsg, raw)
	}
}

func TestAttachment(t *testing.T) {
	a := Attachment{Name: "rules.json", Data: []byte(`[{"name":"test"}]`)}
	if a.Name != "rules.json" {
		t.Errorf("expected rules.json, got %s", a.Name)
	}
	if !bytes.Contains(a.Data, []byte("test")) {
		t.Error("data not preserved")
	}
}
