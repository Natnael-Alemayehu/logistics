package mail

import (
	"context"
	"io"
	"mime/quotedprintable"
	"net"
	"net/textproto"
	"strings"
	"testing"
	"time"

	"github.com/natnael-alemayehu/kiflet/pkg/console"
)

// fakeSMTP is just enough of a mail server to receive one message.
type fakeSMTP struct {
	addr string
	got  chan received
}

type received struct {
	from, to string
	data     string
}

func startFakeSMTP(t *testing.T) *fakeSMTP {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	f := &fakeSMTP{addr: ln.Addr().String(), got: make(chan received, 1)}
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		tp := textproto.NewConn(conn)
		var r received
		_ = tp.PrintfLine("220 fake ESMTP")
		for {
			line, err := tp.ReadLine()
			if err != nil {
				return
			}
			cmd := strings.ToUpper(strings.SplitN(line, " ", 2)[0])
			switch {
			case cmd == "EHLO" || cmd == "HELO":
				_ = tp.PrintfLine("250-fake\r\n250 AUTH PLAIN")
			case cmd == "AUTH":
				_ = tp.PrintfLine("235 ok")
			case strings.HasPrefix(line, "MAIL FROM:"):
				r.from = strings.Trim(strings.TrimPrefix(line, "MAIL FROM:"), "<>")
				_ = tp.PrintfLine("250 ok")
			case strings.HasPrefix(line, "RCPT TO:"):
				r.to = strings.Trim(strings.TrimPrefix(line, "RCPT TO:"), "<>")
				_ = tp.PrintfLine("250 ok")
			case cmd == "DATA":
				_ = tp.PrintfLine("354 go ahead")
				b, _ := tp.ReadDotBytes()
				r.data = string(b)
				_ = tp.PrintfLine("250 queued")
			case cmd == "QUIT":
				_ = tp.PrintfLine("221 bye")
				f.got <- r
				return
			default:
				_ = tp.PrintfLine("502 unknown")
			}
		}
	}()
	return f
}

func TestSMTPDeliversUTF8Text(t *testing.T) {
	srv := startFakeSMTP(t)
	s, err := New(Config{Provider: "smtp", SMTP: SMTP{Addr: srv.addr, Username: "u", Password: "p", From: "Kiflet <no-reply@kiflet.et>"}})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	link := "https://kiflet.example.et/reset/" + strings.Repeat("A", 80)
	err = s.Send(ctx, Message{To: "Almaz <almaz@example.et>", Subject: "ሰላም: reset your password", Text: "Hello,\nOpen " + link + "\n"})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	r := <-srv.got
	if r.from != "no-reply@kiflet.et" || r.to != "almaz@example.et" {
		t.Errorf("envelope from %q to %q", r.from, r.to)
	}
	head, body, _ := strings.Cut(r.data, "\n\n")
	if !strings.Contains(head, "Subject: =?utf-8?q?") || !strings.Contains(head, "charset=utf-8") {
		t.Errorf("headers:\n%s", head)
	}
	decoded, err := io.ReadAll(quotedprintable.NewReader(strings.NewReader(body)))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(decoded), link) {
		t.Errorf("the link did not survive encoding:\n%s", decoded)
	}
}

func TestSMTPRequiresAServer(t *testing.T) {
	if _, err := New(Config{Provider: "smtp", SMTP: SMTP{From: "a@b.et"}}); err != ErrNotConfigured {
		t.Fatalf("err = %v", err)
	}
	if _, err := New(Config{Provider: "smtp", SMTP: SMTP{Addr: "mail:587", From: "not an address"}}); err == nil {
		t.Fatal("accepted an invalid MAIL_FROM")
	}
	if _, err := New(Config{Provider: "carrier-pigeon"}); err == nil {
		t.Fatal("accepted an unknown provider")
	}
}

func TestMockPrintsToTheConsole(t *testing.T) {
	c := console.New(nil)
	s, _ := New(Config{Provider: "mock", Console: c})
	if err := s.Send(context.Background(), Message{To: "almaz@example.et", Subject: "Hi", Text: "Body"}); err != nil {
		t.Fatal(err)
	}
	it, ok := c.Last("email", "almaz@example.et")
	if !ok || it.Title != "Hi" || it.Body != "Body" {
		t.Fatalf("console has %+v", it)
	}
}
