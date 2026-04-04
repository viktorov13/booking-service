package auth

import (
	"errors"
	"time"

	"room-booking-service/internal/domain"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Claims struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

type JWTManager struct {
	secret []byte
	ttl    time.Duration
}

func NewJWTManager(secret string, ttl time.Duration) *JWTManager {
	return &JWTManager{
		secret: []byte(secret),
		ttl:    ttl,
	}
}

func (m *JWTManager) Generate(userID uuid.UUID, role domain.Role) (string, error) {
	now := time.Now().UTC()
	claims := Claims{
		UserID: userID.String(),
		Role:   string(role),
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.ttl)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

func (m *JWTManager) Parse(token string) (domain.AuthUser, error) {
	parsedToken, err := jwt.ParseWithClaims(token, &Claims{}, func(_ *jwt.Token) (any, error) {
		return m.secret, nil
	})
	if err != nil {
		return domain.AuthUser{}, err
	}

	claims, ok := parsedToken.Claims.(*Claims)
	if !ok || !parsedToken.Valid {
		return domain.AuthUser{}, errors.New("invalid token claims")
	}

	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return domain.AuthUser{}, err
	}

	role := domain.Role(claims.Role)
	if !role.Valid() {
		return domain.AuthUser{}, errors.New("invalid role in token")
	}

	return domain.AuthUser{
		ID:   userID,
		Role: role,
	}, nil
}
