package auth

import (
	"time"

	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/google/uuid"
)

type RegisterRequestJSON struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginRequestJSON struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type RefreshRequestJSON struct {
	RefreshToken string `json:"refresh_token"`
}

type AuthResponseJSON struct {
	AccessToken  string   `json:"access_token"`
	RefreshToken string   `json:"refresh_token"`
	User         UserJSON `json:"user"`
}

type UserJSON struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

func toUserJSON(u *domain.User) UserJSON {
	return UserJSON{
		ID:        u.ID,
		Name:      u.Name,
		Email:     u.Email,
		CreatedAt: u.CreatedAt,
	}
}
