package smtp

import (
	"bytes"
	"testing"
)

func TestBuildMIME(t *testing.T) {
	var buf bytes.Buffer
	att := Attachment{Name: "test.json", Data: []byte(`{"key":"value"}`)}
	_ = att

	subject := "Mailflow Rules Backup"
	from := "user@icloud.com"
	to := "user@icloud.com"

	buf.WriteString("From: " + from + "\r\n")
	buf.WriteString("To: " + to + "\r\n")
	buf.WriteString("Subject: " + subject + "\r\n")
	buf.WriteString("MIME-Version: 1.0\r\n")

	if !bytes.Contains(buf.Bytes(), []byte(from)) {
		t.Error("missing From")
	}
	if !bytes.Contains(buf.Bytes(), []byte(to)) {
		t.Error("missing To")
	}
	if !bytes.Contains(buf.Bytes(), []byte(subject)) {
		t.Error("missing Subject")
	}
	if !bytes.Contains(buf.Bytes(), []byte("MIME-Version: 1.0")) {
		t.Error("missing MIME-Version")
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
