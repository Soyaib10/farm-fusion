package auth

import "github.com/Soyaib10/farm-fusion/internal/domain"

type RegisterCommand struct {
	Name     string
	Email    string
	Password string
}

type LoginCommand struct {
	Email    string
	Password string
}

type AuthResponse struct {
	AccessToken  string
	RefreshToken string
	User         *domain.User
}
