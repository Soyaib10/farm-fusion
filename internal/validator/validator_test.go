package validator

import (
	"math"
	"strings"
	"testing"
)

func TestStrongPassword(t *testing.T) {
	tests := []struct {
		name string
		pw   string
		want bool
	}{
		{"too short", "Ab1!", false},
		{"exceeds bcrypt 72-byte limit", strings.Repeat("a", 72) + "1", false},
		{"letters only", "abcdefghij", false},
		{"digits only", "1234567890", false},
		{"contains newline control char", "SecurePass1\n", false},
		{"valid strong password", "StrongPass123!", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, _ := StrongPassword(tc.pw)
			if got != tc.want {
				t.Fatalf("StrongPassword(%q) = %v, want %v", tc.pw, got, tc.want)
			}
		})
	}
}

func TestBetweenAndControlChars(t *testing.T) {
	if Between(math.NaN(), -90, 90) {
		t.Fatal("Between should reject NaN")
	}
	if Between(math.Inf(1), -90, 90) {
		t.Fatal("Between should reject +Inf")
	}
	if !Between(0, -90, 90) {
		t.Fatal("Between should accept 0 within [-90, 90]")
	}
	if NoControlChars("Farm\r\nBcc: evil@example.com") {
		t.Fatal("NoControlChars should reject CRLF")
	}
	if !NoControlChars("Green Valley Farm") {
		t.Fatal("NoControlChars should accept normal text")
	}
}
