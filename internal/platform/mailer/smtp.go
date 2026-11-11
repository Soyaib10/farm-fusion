package mailer

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"html/template"
	"mime"
	"net"
	"net/mail"
	netsmtp "net/smtp"
	"strconv"
	"strings"
	"time"

	"github.com/Soyaib10/farm-fusion/internal/config"
	"github.com/Soyaib10/farm-fusion/internal/domain"
)

// Sender delivers HTML weather alert emails over SMTP with CRLF protection and TLS enforcement.
type Sender struct {
	host       string
	port       int
	user       string
	password   string
	from       string
	requireTLS bool
	timeout    time.Duration
}

// NewSender constructs an SMTP Sender from configuration.
func NewSender(cfg config.SMTPConfig) *Sender {
	from := cfg.From
	if from == "" {
		from = cfg.User
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &Sender{
		host:       strings.TrimSpace(cfg.Host),
		port:       cfg.Port,
		user:       cfg.User,
		password:   cfg.Password,
		from:       strings.TrimSpace(from),
		requireTLS: cfg.RequireTLS,
		timeout:    timeout,
	}
}

// SendWeatherNotification renders and sends a weather notification email, returning the rendered HTML.
func (s *Sender) SendWeatherNotification(ctx context.Context, payload *domain.NotificationPayload) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if s.host == "" {
		return "", errors.New("smtp host is required")
	}

	fromAddr, err := parseSafeAddress(s.from)
	if err != nil {
		return "", fmt.Errorf("invalid smtp from address: %w", err)
	}
	toAddr, err := parseSafeAddress(payload.UserEmail)
	if err != nil {
		return "", fmt.Errorf("invalid recipient email address: %w", err)
	}

	content, err := renderWeatherEmail(payload)
	if err != nil {
		return "", err
	}

	msg := BuildMessage(fromAddr, toAddr, SubjectForFarm(payload.FarmName), content)
	if err := s.send(ctx, fromAddr.Address, toAddr.Address, []byte(msg)); err != nil {
		return content, err
	}
	return content, nil
}

func (s *Sender) send(ctx context.Context, from, to string, msg []byte) error {
	sendCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	addr := net.JoinHostPort(s.host, strconv.Itoa(s.port))
	dialer := &net.Dialer{Timeout: s.timeout}

	tlsCfg := &tls.Config{
		ServerName: s.host,
		MinVersion: tls.VersionTLS12,
	}

	var (
		conn net.Conn
		err  error
	)

	// Port 465 uses implicit SMTPS; other ports upgrade via STARTTLS.
	if s.port == 465 {
		tlsDialer := &tls.Dialer{NetDialer: dialer, Config: tlsCfg}
		conn, err = tlsDialer.DialContext(sendCtx, "tcp", addr)
	} else {
		conn, err = dialer.DialContext(sendCtx, "tcp", addr)
	}
	if err != nil {
		return fmt.Errorf("dial smtp server: %w", err)
	}
	defer conn.Close()

	if deadline, ok := sendCtx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}

	client, err := netsmtp.NewClient(conn, s.host)
	if err != nil {
		return fmt.Errorf("create smtp client: %w", err)
	}
	defer client.Close()

	if s.port != 465 {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(tlsCfg); err != nil {
				return fmt.Errorf("smtp starttls: %w", err)
			}
		} else if s.requireTLS {
			return errors.New("smtp server does not support STARTTLS while SMTP_REQUIRE_TLS is enabled")
		}
	}

	if s.user != "" || s.password != "" {
		auth := netsmtp.PlainAuth("", s.user, s.password, s.host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}

	if err := client.Mail(from); err != nil {
		return fmt.Errorf("smtp mail from: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("smtp rcpt to: %w", err)
	}

	wc, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err := wc.Write(msg); err != nil {
		_ = wc.Close()
		return fmt.Errorf("smtp write body: %w", err)
	}
	if err := wc.Close(); err != nil {
		return fmt.Errorf("smtp close data: %w", err)
	}

	return client.Quit()
}

// SubjectForFarm generates a sanitized subject line immune to header injection.
func SubjectForFarm(farmName string) string {
	clean := SanitizeHeaderValue(farmName)
	return fmt.Sprintf("Weather alert for %s", clean)
}

// SanitizeHeaderValue strips CR, LF, and NUL characters to prevent SMTP header injection.
func SanitizeHeaderValue(val string) string {
	replacer := strings.NewReplacer("\r", " ", "\n", " ", "\x00", "")
	return strings.TrimSpace(replacer.Replace(val))
}

func parseSafeAddress(raw string) (*mail.Address, error) {
	if strings.ContainsAny(raw, "\r\n\x00") {
		return nil, errors.New("address contains forbidden control characters")
	}
	addr, err := mail.ParseAddress(strings.TrimSpace(raw))
	if err != nil {
		return nil, err
	}
	if strings.ContainsAny(addr.Address, "\r\n\x00") {
		return nil, errors.New("parsed address contains forbidden control characters")
	}
	return addr, nil
}

// BuildMessage formats RFC 5322 headers and HTML body safely using encoded header values.
func BuildMessage(from, to *mail.Address, subjectLine, htmlBody string) string {
	encodedSubject := mime.QEncoding.Encode("utf-8", SanitizeHeaderValue(subjectLine))
	headers := []string{
		"From: " + from.String(),
		"To: " + to.String(),
		"Subject: " + encodedSubject,
		"MIME-Version: 1.0",
		`Content-Type: text/html; charset="UTF-8"`,
	}
	return strings.Join(headers, "\r\n") + "\r\n\r\n" + htmlBody
}

func renderWeatherEmail(payload *domain.NotificationPayload) (string, error) {
	var buf bytes.Buffer
	if err := weatherEmailTemplate.Execute(&buf, struct {
		Payload *domain.NotificationPayload
		SentAt  string
	}{
		Payload: payload,
		SentAt:  payload.Timestamp.Format(time.RFC1123),
	}); err != nil {
		return "", fmt.Errorf("render weather email: %w", err)
	}
	return buf.String(), nil
}

var weatherEmailTemplate = template.Must(template.New("weather-email").Parse(`<!doctype html>
<html>
<body style="font-family: Arial, sans-serif; color: #1f2933; line-height: 1.5;">
  <h2>Weather alert for {{.Payload.FarmName}}</h2>
  <p>Generated at {{.SentAt}}</p>
  <p>
    Forecast summary: {{printf "%.1f" .Payload.ForecastSummary.TempMin}}&deg;C to
    {{printf "%.1f" .Payload.ForecastSummary.TempMax}}&deg;C,
    {{printf "%.1f" .Payload.ForecastSummary.TotalRainfall}} mm total rainfall.
  </p>
  {{range .Payload.Alerts}}
    <section style="margin-top: 16px; padding: 12px; border: 1px solid #d9e2ec; border-radius: 8px;">
      <h3 style="margin-top: 0;">{{.AlertType}} {{.Threshold.Operator}} {{printf "%.1f" .Threshold.Value}}</h3>
      <ul>
        {{range .Ranges}}
          <li>
            {{.Start.Format "Jan 02 15:04"}} to {{.End.Format "Jan 02 15:04"}}
            ({{.DurationHours}} hours), values {{printf "%.1f" .MinValue}} to {{printf "%.1f" .MaxValue}}
          </li>
        {{end}}
      </ul>
    </section>
  {{end}}
</body>
</html>`))
