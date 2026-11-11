package auth

import (
	"fmt"
	"time"

	"github.com/Soyaib10/farm-fusion/internal/config"
	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// AccessClaims defines the custom JWT claims embedded in access tokens.
type AccessClaims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

// TokenManager issues and strictly validates HS256 JWT access tokens.
type TokenManager struct {
	secret   []byte
	issuer   string
	audience string
	ttl      time.Duration
}

// NewTokenManager constructs a TokenManager from AuthConfig.
func NewTokenManager(cfg config.AuthConfig) *TokenManager {
	return &TokenManager{
		secret:   []byte(cfg.JWTSecret),
		issuer:   cfg.JWTIssuer,
		audience: cfg.JWTAudience,
		ttl:      cfg.AccessTokenDuration,
	}
}

// IssueAccessToken creates a signed HS256 JWT for u.
func (m *TokenManager) IssueAccessToken(u *domain.User) (string, error) {
	now := time.Now().UTC()
	role := u.Role
	if role == "" {
		role = domain.RoleUser
	}

	claims := AccessClaims{
		Role: string(role),
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.NewString(),
			Subject:   u.ID.String(),
			Issuer:    m.issuer,
			Audience:  jwt.ClaimStrings{m.audience},
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.ttl)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(m.secret)
	if err != nil {
		return "", fmt.Errorf("sign access token: %w", err)
	}
	return signed, nil
}

// VerifyAccessToken validates signature algorithm, issuer, audience, and expiration,
// returning the authenticated domain.Actor.
func (m *TokenManager) VerifyAccessToken(rawToken string) (domain.Actor, error) {
	var claims AccessClaims

	token, err := jwt.ParseWithClaims(
		rawToken,
		&claims,
		func(t *jwt.Token) (any, error) {
			return m.secret, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(m.issuer),
		jwt.WithAudience(m.audience),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
	)
	if err != nil || !token.Valid {
		return domain.Actor{}, domain.ErrUnauthorized
	}

	userID, err := uuid.Parse(claims.Subject)
	if err != nil || userID == uuid.Nil {
		return domain.Actor{}, domain.ErrUnauthorized
	}

	role := domain.Role(claims.Role)
	if role == "" {
		role = domain.RoleUser
	}
	if !role.IsValid() {
		return domain.Actor{}, domain.ErrUnauthorized
	}

	return domain.Actor{
		ID:   userID,
		Role: role,
	}, nil
}
