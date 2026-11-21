package notify

import (
	"context"
	"crypto/tls"
	"fmt"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

// Email sends plain-text mail over SMTP.
type Email struct {
	Host     string
	Port     int
	Username string // empty = no authentication
	Password string
	From     string
	To       []string
	// Security is "starttls" (default, usually port 587), "tls" (implicit TLS,
	// usually port 465) or "none" (plain, only for a trusted local relay).
	Security string
}

func (e *Email) Name() string { return "email" }

func (e *Email) Send(ctx context.Context, m Message) error {
	from, err := mail.ParseAddress(e.From)
	if err != nil {
		return fmt.Errorf("from address: %w", err)
	}
	var to []string
	for _, t := range e.To {
		a, err := mail.ParseAddress(t)
		if err != nil {
			return fmt.Errorf("to address %q: %w", t, err)
		}
		to = append(to, a.Address)
	}
	if len(to) == 0 {
		return fmt.Errorf("no recipients")
	}

	addr := net.JoinHostPort(e.Host, strconv.Itoa(e.Port))
	d := net.Dialer{Timeout: 15 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return err
	}
	conn.SetDeadline(time.Now().Add(45 * time.Second))
	security := strings.ToLower(e.Security)
	if security == "tls" {
		conn = tls.Client(conn, &tls.Config{ServerName: e.Host})
	}
	c, err := smtp.NewClient(conn, e.Host)
	if err != nil {
		conn.Close()
		return err
	}
	defer c.Close()

	if security == "" || security == "starttls" {
		if ok, _ := c.Extension("STARTTLS"); !ok {
			return fmt.Errorf("server does not offer STARTTLS (set security = \"none\" only for a trusted local relay)")
		}
		if err := c.StartTLS(&tls.Config{ServerName: e.Host}); err != nil {
			return err
		}
	}
	if e.Username != "" {
		if err := c.Auth(smtp.PlainAuth("", e.Username, e.Password, e.Host)); err != nil {
			return err
		}
	}
	if err := c.Mail(from.Address); err != nil {
		return err
	}
	for _, t := range to {
		if err := c.Rcpt(t); err != nil {
			return err
		}
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(buildMessage(from, to, m, time.Now())); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}

func buildMessage(from *mail.Address, to []string, m Message, now time.Time) []byte {
	oneLine := strings.NewReplacer("\r", " ", "\n", " ").Replace(m.Subject) // no header injection
	body := strings.ReplaceAll(strings.ReplaceAll(m.Body, "\r\n", "\n"), "\n", "\r\n")
	var b strings.Builder
	b.WriteString("From: " + from.String() + "\r\n")
	b.WriteString("To: " + strings.Join(to, ", ") + "\r\n")
	b.WriteString("Subject: " + mime.QEncoding.Encode("utf-8", oneLine) + "\r\n")
	b.WriteString("Date: " + now.Format(time.RFC1123Z) + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	b.WriteString("\r\n")
	b.WriteString(body + "\r\n")
	return []byte(b.String())
}
