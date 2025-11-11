package domain

import (
	"errors"
	"regexp"
	"time"
)

type User struct {
	ID        string
	Name      string
	Email     string
	CreatedAt time.Time
}

func (u *User) Validate() error {
	if u.ID == "" {
		return errors.New("name is required")
	}

	if u.Email == "" {
		return errors.New("email is required")
	}

	// simple email regex check
	re := regexp.MustCompile(`^[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}$`)
	if !re.MatchString(u.Email) {
		return errors.New("invalid email format")
	}

	return nil
}
