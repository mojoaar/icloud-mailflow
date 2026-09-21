package smtp

import (
	"bytes"
	"fmt"
	"log/slog"
	"mime"
	"mime/multipart"
	"net/smtp"
	"net/textproto"
	"strings"
	"time"
)

type Attachment struct {
	Name string
	Data []byte
}

const smtpHost = "smtp.mail.me.com:587"

func Send(to, from, password, subject, body string, attachments ...Attachment) error {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("From: %s\r\n", from))
	buf.WriteString(fmt.Sprintf("To: %s\r\n", to))
	buf.WriteString(fmt.Sprintf("Subject: %s\r\n", mime.QEncoding.Encode("utf-8", subject)))
	buf.WriteString(fmt.Sprintf("Date: %s\r\n", time.Now().Format(time.RFC1123Z)))
	buf.WriteString("MIME-Version: 1.0\r\n")

	boundary := fmt.Sprintf("mailflow-%d", time.Now().UnixNano())
	mp := multipart.NewWriter(&buf)

	mp.SetBoundary(boundary)
	buf.WriteString(fmt.Sprintf("Content-Type: multipart/mixed; boundary=\"%s\"\r\n\r\n", boundary))

	textHeader := textproto.MIMEHeader{}
	textHeader.Set("Content-Type", "text/plain; charset=utf-8")
	tw, err := mp.CreatePart(textHeader)
	if err != nil {
		return err
	}
	if _, err := tw.Write([]byte(body)); err != nil {
		return err
	}

	for _, a := range attachments {
		attHeader := textproto.MIMEHeader{}
		attHeader.Set("Content-Type", "application/json; charset=utf-8")
		attHeader.Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", a.Name))
		attHeader.Set("Content-Transfer-Encoding", "binary")
		aw, err := mp.CreatePart(attHeader)
		if err != nil {
			return err
		}
		if _, err := aw.Write(a.Data); err != nil {
			return err
		}
	}

	if err := mp.Close(); err != nil {
		return err
	}

	server, _, _ := strings.Cut(smtpHost, ":")
	auth := smtp.PlainAuth("", from, password, server)
	if err := smtp.SendMail(smtpHost, auth, from, []string{to}, buf.Bytes()); err != nil {
		return err
	}
	slog.Debug("smtp send", "to", to, "subject", subject)
	return nil
}

func SendRaw(to, from, password string, raw []byte) error {
	server, _, _ := strings.Cut(smtpHost, ":")
	auth := smtp.PlainAuth("", from, password, server)
	if err := smtp.SendMail(smtpHost, auth, from, []string{to}, raw); err != nil {
		return err
	}
	slog.Debug("smtp send raw", "to", to)
	return nil
}
