package logger

import (
	"bytes"
	"strings"
	"testing"
)

func TestLoggerRedactsSensitiveProperties(t *testing.T) {
	var buf bytes.Buffer
	log := New(&buf, LevelInfo)

	log.Info("auth check", map[string]string{
		"user_id":       "123",
		"password":      "SuperSecret123!",
		"jwt_secret":    "raw-jwt-key",
		"api_key":       "owm-secret",
		"Authorization": "Bearer eyJ...",
	})

	out := buf.String()
	for _, secret := range []string{"SuperSecret123!", "raw-jwt-key", "owm-secret", "Bearer eyJ..."} {
		if strings.Contains(out, secret) {
			t.Fatalf("logger leaked sensitive value %q in output: %s", secret, out)
		}
	}
	if !strings.Contains(out, "[REDACTED]") {
		t.Fatalf("expected [REDACTED] marker in output: %s", out)
	}
}
