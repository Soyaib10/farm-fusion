package mailer

import (
	"bufio"
	"context"
	"net"
	"net/mail"
	"strings"
	"testing"
	"time"

	"github.com/Soyaib10/farm-fusion/internal/config"
	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/google/uuid"
)

// fakeSMTPServer accepts one message and then drops the connection when the client sends QUIT.
func fakeSMTPServer(t *testing.T) (host string, port int, accepted <-chan string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	out := make(chan string, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		rw := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))
		reply := func(s string) { _, _ = rw.WriteString(s + "\r\n"); _ = rw.Flush() }

		reply("220 fake ESMTP")
		var body strings.Builder
		inData := false
		for {
			line, err := rw.ReadString('\n')
			if err != nil {
				return
			}
			if inData {
				if line == ".\r\n" {
					inData = false
					reply("250 OK queued")
					out <- body.String()
					continue
				}
				body.WriteString(line)
				continue
			}
			switch cmd := strings.ToUpper(strings.TrimSpace(line)); {
			case strings.HasPrefix(cmd, "EHLO"), strings.HasPrefix(cmd, "HELO"):
				reply("250 fake")
			case strings.HasPrefix(cmd, "MAIL"), strings.HasPrefix(cmd, "RCPT"):
				reply("250 OK")
			case cmd == "DATA":
				inData = true
				reply("354 go ahead")
			case cmd == "QUIT":
				return // Drop the connection instead of answering 221.
			default:
				reply("502 not implemented")
			}
		}
	}()

	addr := ln.Addr().(*net.TCPAddr)
	return addr.IP.String(), addr.Port, out
}

func TestSendSucceedsWhenQuitFailsAfterAcceptedData(t *testing.T) {
	host, port, accepted := fakeSMTPServer(t)
	var quitErr error
	sender := NewSender(config.SMTPConfig{
		Host:       host,
		Port:       port,
		From:       "noreply@farmfusion.com",
		RequireTLS: false,
		Timeout:    5 * time.Second,
	}, func(err error) { quitErr = err })

	payload := &domain.NotificationPayload{
		NotificationID: uuid.New(),
		UserEmail:      "farmer@example.com",
		FarmName:       "North Field",
		Timestamp:      time.Now().UTC(),
	}
	if _, err := sender.SendWeatherNotification(context.Background(), payload); err != nil {
		t.Fatalf("SendWeatherNotification() = %v, want nil after the server accepted DATA", err)
	}
	select {
	case msg := <-accepted:
		if !strings.Contains(msg, "North Field") {
			t.Fatalf("unexpected message body: %q", msg)
		}
	default:
		t.Fatal("server did not receive the message")
	}
	if quitErr == nil {
		t.Fatal("expected the QUIT failure to be reported to onQuitError")
	}
}

func TestSMTPHeaderInjectionPrevention(t *testing.T) {
	maliciousFarmName := "North Field\r\nBcc: victim@target.com\r\nX-Injected: pwned"
	subj := SubjectForFarm(maliciousFarmName)
	if strings.ContainsAny(subj, "\r\n") {
		t.Fatalf("SubjectForFarm failed to strip CRLF: %q", subj)
	}

	from := &mail.Address{Address: "noreply@farmfusion.com"}
	to := &mail.Address{Address: "farmer@example.com"}
	rawMsg := BuildMessage(from, to, subj, "<p>Hello</p>")

	headerSection, _, ok := strings.Cut(rawMsg, "\r\n\r\n")
	if !ok {
		t.Fatal("expected header/body separator in message")
	}
	for _, line := range strings.Split(headerSection, "\r\n") {
		if strings.HasPrefix(strings.ToLower(line), "bcc:") || strings.HasPrefix(strings.ToLower(line), "x-injected:") {
			t.Fatalf("SMTP header injection succeeded: %q", line)
		}
	}

	if _, err := parseSafeAddress("farmer@example.com\r\nBcc: leak@evil.com"); err == nil {
		t.Fatal("expected parseSafeAddress to reject CRLF in email address")
	}
}

func TestRenderWeatherEmailEscapesHTML(t *testing.T) {
	payload := &domain.NotificationPayload{
		NotificationID: uuid.New(),
		FarmID:         uuid.New(),
		UserID:         uuid.New(),
		UserEmail:      "farmer@example.com",
		FarmName:       `<script>alert("xss")</script>`,
		Timestamp:      time.Now().UTC(),
	}

	rendered, err := renderWeatherEmail(payload)
	if err != nil {
		t.Fatalf("renderWeatherEmail unexpected error: %v", err)
	}
	if strings.Contains(rendered, `<script>alert("xss")</script>`) {
		t.Fatal("expected HTML template to escape <script> tag in FarmName")
	}
}
