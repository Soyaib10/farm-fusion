package mailer

import (
	"net/mail"
	"strings"
	"testing"
	"time"

	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/google/uuid"
)

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
