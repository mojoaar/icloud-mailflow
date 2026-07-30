package smtp

import (
	"bytes"
	"fmt"
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
	tw, _ := mp.CreatePart(textHeader)
	tw.Write([]byte(body))

	for _, a := range attachments {
		attHeader := textproto.MIMEHeader{}
		attHeader.Set("Content-Type", "application/json; charset=utf-8")
		attHeader.Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", a.Name))
		attHeader.Set("Content-Transfer-Encoding", "binary")
		aw, _ := mp.CreatePart(attHeader)
		aw.Write(a.Data)
	}

	mp.Close()

	host := "smtp.mail.me.com:587"
	server, _, _ := strings.Cut(host, ":")
	auth := smtp.PlainAuth("", from, password, server)
	return smtp.SendMail(host, auth, from, []string{to}, buf.Bytes())
}
