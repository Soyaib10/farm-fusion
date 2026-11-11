package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const bcryptCost = 12

// Role represents an authorization role assigned to a user.
type Role string

const (
	RoleUser  Role = "user"
	RoleAdmin Role = "admin"
)

// IsValid reports whether r is a supported user role.
func (r Role) IsValid() bool {
	return r == RoleUser || r == RoleAdmin
}

// Actor represents the authenticated principal performing an operation.
type Actor struct {
	ID   uuid.UUID
	Role Role
}

// IsAdmin reports whether the actor holds administrative privileges.
func (a Actor) IsAdmin() bool {
	return a.Role == RoleAdmin
}

// CanAccessUser reports whether the actor is permitted to read or mutate targetUserID.
func (a Actor) CanAccessUser(targetUserID uuid.UUID) bool {
	return a.ID != uuid.Nil && (a.ID == targetUserID || a.IsAdmin())
}

// dummyPasswordHash is precomputed at package init so failed lookups still perform a
// full bcrypt comparison, mitigating timing-based user enumeration on login.
var dummyPasswordHash []byte

func init() {
	h, err := bcrypt.GenerateFromPassword([]byte("farm-fusion-dummy-constant-time-secret"), bcryptCost)
	if err != nil {
		panic("failed to initialize dummy bcrypt hash: " + err.Error())
	}
	dummyPasswordHash = h
}

// User represents a registered platform user.
type User struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	Email        string    `json:"email"`
	Role         Role      `json:"role"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

// SetPassword hashes plaintext using bcrypt and stores the resulting hash on u.
func (u *User) SetPassword(password string) error {
	if len(password) > 72 {
		return errors.New("password must not exceed 72 bytes")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return err
	}
	u.PasswordHash = string(hash)
	return nil
}

// CheckPassword verifies password against u.PasswordHash.
func (u *User) CheckPassword(password string) bool {
	if len(password) > 72 || u.PasswordHash == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) == nil
}

// SimulatePasswordCheck performs a dummy bcrypt comparison to equalize response times
// when a login attempt references a non-existent email address.
func SimulatePasswordCheck(password string) {
	if len(password) > 72 {
		password = password[:72]
	}
	_ = bcrypt.CompareHashAndPassword(dummyPasswordHash, []byte(password))
}
