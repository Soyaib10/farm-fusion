package smtp

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"net"
	netsmtp "net/smtp"
	"strings"
	"time"

	"github.com/Soyaib10/farm-fusion/internal/config"
	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/Soyaib10/farm-fusion/internal/usecase/notification"
)

type Sender struct {
	host     string
	port     int
	user     string
	password string
	from     string
}

func NewSender(cfg config.SMTPConfig) notification.EmailSender {
	from := cfg.From
	if from == "" {
		from = cfg.User
	}
	return &Sender{
		host:     cfg.Host,
		port:     cfg.Port,
		user:     cfg.User,
		password: cfg.Password,
		from:     from,
	}
}

func (s *Sender) SendWeatherNotification(ctx context.Context, payload *domain.NotificationPayload) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	if s.host == "" {
		return "", fmt.Errorf("smtp host is required")
	}
	if s.from == "" {
		return "", fmt.Errorf("smtp from address is required")
	}
	if payload.UserEmail == "" {
		return "", fmt.Errorf("recipient email is required")
	}

	content, err := renderWeatherEmail(payload)
	if err != nil {
		return "", err
	}

	message := buildMessage(s.from, payload.UserEmail, subject(payload), content)
	addr := net.JoinHostPort(s.host, fmt.Sprintf("%d", s.port))

	var auth netsmtp.Auth
	if s.user != "" || s.password != "" {
		auth = netsmtp.PlainAuth("", s.user, s.password, s.host)
	}

	if err := netsmtp.SendMail(addr, auth, s.from, []string{payload.UserEmail}, []byte(message)); err != nil {
		return content, fmt.Errorf("send smtp mail: %w", err)
	}
	return content, nil
}

func subject(payload *domain.NotificationPayload) string {
	return fmt.Sprintf("Weather alert for %s", payload.FarmName)
}

func buildMessage(from, to, subjectLine, html string) string {
	headers := []string{
		"From: " + from,
		"To: " + to,
		"Subject: " + subjectLine,
		"MIME-Version: 1.0",
		`Content-Type: text/html; charset="UTF-8"`,
	}
	return strings.Join(headers, "\r\n") + "\r\n\r\n" + html
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
