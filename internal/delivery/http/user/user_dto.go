package user

import (
	"time"

	"github.com/Soyaib10/farm-fusion/internal/domain"
)

type UpsertUserJSON struct {
	Name  *string `json:"name,omitempty"  validate:"omitempty,min=1"`
	Email *string `json:"email,omitempty" validate:"omitempty,email"`
}

type UserResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	CreatedAt string `json:"created_at"`
}

func toUserResponse(u *domain.User) UserResponse {
	return UserResponse{
		ID:        u.ID.String(),
		Name:      u.Name,
		Email:     u.Email,
		CreatedAt: u.CreatedAt.Format(time.RFC3339),
	}
}
