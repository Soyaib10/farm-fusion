package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/Soyaib10/farm-fusion/internal/app"
	"github.com/Soyaib10/farm-fusion/internal/config"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type JWTMiddleware struct {
	app *app.Application
	cfg *config.Config
}

func NewJWTMiddleware(app *app.Application, cfg *config.Config) *JWTMiddleware {
	return &JWTMiddleware{
		app: app,
		cfg: cfg,
	}
}

func (m *JWTMiddleware) RequireAuthenticatedUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			m.app.InvalidAuthenticationTokenResponse(w, r)
			return
		}

		headerParts := strings.Split(authHeader, " ")
		if len(headerParts) != 2 || headerParts[0] != "Bearer" {
			m.app.InvalidAuthenticationTokenResponse(w, r)
			return
		}

		tokenString := headerParts[1]

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(m.cfg.JWTSecret), nil
		})

		if err != nil {
			m.app.InvalidAuthenticationTokenResponse(w, r)
			return
		}

		if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
			userIDStr, ok := claims["sub"].(string)
			if !ok {
				m.app.InvalidAuthenticationTokenResponse(w, r)
				return
			}

			userID, err := uuid.Parse(userIDStr)
			if err != nil {
				m.app.InvalidAuthenticationTokenResponse(w, r)
				return
			}

			ctx := context.WithValue(r.Context(), "user_id", userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		} else {
			m.app.InvalidAuthenticationTokenResponse(w, r)
			return
		}
	})
}
