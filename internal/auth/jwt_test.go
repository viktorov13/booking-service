package auth

import (
	"testing"
	"time"

	"room-booking-service/internal/domain"

	"github.com/google/uuid"
)

func TestJWTManagerGenerateAndParse(t *testing.T) {
	t.Parallel()

	manager := NewJWTManager("secret", time.Hour)
	userID := uuid.New()

	token, err := manager.Generate(userID, domain.RoleUser)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	actor, err := manager.Parse(token)
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}

	if actor.ID != userID {
		t.Fatalf("expected user id %s, got %s", userID, actor.ID)
	}
	if actor.Role != domain.RoleUser {
		t.Fatalf("expected role %s, got %s", domain.RoleUser, actor.Role)
	}
}

func TestJWTManagerParseInvalidToken(t *testing.T) {
	t.Parallel()

	manager := NewJWTManager("secret", time.Hour)
	if _, err := manager.Parse("invalid-token"); err == nil {
		t.Fatal("expected parse error")
	}
}
