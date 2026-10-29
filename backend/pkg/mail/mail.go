// Package mail sends email: through an SMTP server in production, or to the
// terminal in development.
package mail

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"

	"github.com/natnael-alemayehu/logistics/pkg/console"
)

// Message is a plain-text email to one recipient.
type Message struct {
	To      string
	Subject string
	Text    string
}

// Sender delivers email.
type Sender interface {
	Name() string
	Send(ctx context.Context, m Message) error
}

// Mock prints each email to a console instead of sending it.
type Mock struct{ c *console.Console }

// NewMock returns a sender that prints to c.
func NewMock(c *console.Console) *Mock { return &Mock{c: c} }

func (*Mock) Name() string { return "mock" }

func (m *Mock) Send(_ context.Context, msg Message) error {
	m.c.Print(console.Item{Kind: "email", To: msg.To, Title: msg.Subject, Body: msg.Text})
	return nil
}

// SMTP sends through a mail server. Port 465 uses implicit TLS; on other
// ports the connection is upgraded with STARTTLS when the server offers it,
// and credentials are only sent over TLS or to a server on this machine.
type SMTP struct {
	// Addr is host:port.
	Addr     string
	Username string
	Password string
	// From is the sender, such as "Menged <no-reply@menged.et>".
	From string
}

func (*SMTP) Name() string { return "smtp" }

func (s *SMTP) Send(ctx context.Context, m Message) error {
	from, err := mail.ParseAddress(s.From)
	if err != nil {
		return fmt.Errorf("MAIL_FROM: %w", err)
	}
	to, err := mail.ParseAddress(m.To)
	if err != nil {
		return fmt.Errorf("recipient: %w", err)
	}
	host, port, err := net.SplitHostPort(s.Addr)
	if err != nil {
		return fmt.Errorf("SMTP_ADDR: %w", err)
	}

	d := &net.Dialer{Timeout: 15 * time.Second}
	var conn net.Conn
	if port == "465" {
		conn, err = (&tls.Dialer{NetDialer: d, Config: &tls.Config{ServerName: host}}).DialContext(ctx, "tcp", s.Addr)
	} else {
		conn, err = d.DialContext(ctx, "tcp", s.Addr)
	}
	if err != nil {
		return err
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(time.Minute)
	}
	_ = conn.SetDeadline(deadline)

	c, err := smtp.NewClient(conn, host)
	if err != nil {
		conn.Close()
		return err
	}
	defer c.Close()
	if port != "465" {
		if ok, _ := c.Extension("STARTTLS"); ok {
			if err := c.StartTLS(&tls.Config{ServerName: host}); err != nil {
				return fmt.Errorf("starttls: %w", err)
			}
		}
	}
	if s.Username != "" {
		// PlainAuth refuses to send the password unencrypted to a remote host.
		if err := c.Auth(smtp.PlainAuth("", s.Username, s.Password, host)); err != nil {
			return fmt.Errorf("auth: %w", err)
		}
	}
	if err := c.Mail(from.Address); err != nil {
		return err
	}
	if err := c.Rcpt(to.Address); err != nil {
		return err
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(compose(from, to, m)); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}

// compose renders the message: UTF-8 text, quoted-printable, so Amharic and
// long links survive any relay.
func compose(from, to *mail.Address, m Message) []byte {
	var b bytes.Buffer
	domain := "localhost"
	if i := strings.LastIndex(from.Address, "@"); i >= 0 {
		domain = from.Address[i+1:]
	}
	id := make([]byte, 12)
	_, _ = rand.Read(id)
	fmt.Fprintf(&b, "From: %s\r\n", from.String())
	fmt.Fprintf(&b, "To: %s\r\n", to.String())
	fmt.Fprintf(&b, "Subject: %s\r\n", mime.QEncoding.Encode("utf-8", m.Subject))
	fmt.Fprintf(&b, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))
	fmt.Fprintf(&b, "Message-ID: <%s@%s>\r\n", hex.EncodeToString(id), domain)
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
	b.WriteString("Content-Transfer-Encoding: quoted-printable\r\n\r\n")
	qp := quotedprintable.NewWriter(&b)
	_, _ = qp.Write([]byte(strings.ReplaceAll(m.Text, "\n", "\r\n")))
	_ = qp.Close()
	return b.Bytes()
}

// ErrNotConfigured is returned by New when SMTP is chosen without a server.
var ErrNotConfigured = errors.New("MAIL_PROVIDER is smtp but SMTP_ADDR is not set")

// Config selects and configures a sender.
type Config struct {
	Provider string // "mock" or "smtp"
	SMTP     SMTP
	Console  *console.Console
}

// New builds the configured sender.
func New(cfg Config) (Sender, error) {
	switch cfg.Provider {
	case "", "mock":
		return NewMock(cfg.Console), nil
	case "smtp":
		if cfg.SMTP.Addr == "" {
			return nil, ErrNotConfigured
		}
		if _, err := mail.ParseAddress(cfg.SMTP.From); err != nil {
			return nil, fmt.Errorf("MAIL_FROM %q: %w", cfg.SMTP.From, err)
		}
		s := cfg.SMTP
		return &s, nil
	}
	return nil, fmt.Errorf("unknown MAIL_PROVIDER %q (want mock or smtp)", cfg.Provider)
}
