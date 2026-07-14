package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestPasswordHashAndCheck(t *testing.T) {
	hash, err := HashPassword("s3cret-password")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if hash == "s3cret-password" {
		t.Fatal("password stored in plaintext")
	}
	if !CheckPassword(hash, "s3cret-password") {
		t.Error("correct password rejected")
	}
	if CheckPassword(hash, "wrong") {
		t.Error("wrong password accepted")
	}
}

func TestAccessTokenRoundTrip(t *testing.T) {
	issuer := NewTokenIssuer("test-secret", 15*time.Minute, time.Hour)
	userID, orgID := uuid.New(), uuid.New()

	tok, err := issuer.IssueAccessToken(userID, orgID, "user@example.com")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	claims, err := issuer.ParseAccessToken(tok)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if claims.Subject != userID.String() {
		t.Errorf("subject: got %s want %s", claims.Subject, userID)
	}
	if claims.OrgID != orgID {
		t.Errorf("org_id: got %s want %s", claims.OrgID, orgID)
	}
	if claims.Email != "user@example.com" {
		t.Errorf("email: got %s", claims.Email)
	}
}

func TestAccessTokenRejectsWrongSecret(t *testing.T) {
	a := NewTokenIssuer("secret-a", time.Minute, time.Hour)
	b := NewTokenIssuer("secret-b", time.Minute, time.Hour)
	tok, _ := a.IssueAccessToken(uuid.New(), uuid.New(), "x@y.com")
	if _, err := b.ParseAccessToken(tok); err == nil {
		t.Error("token forged with a different secret was accepted")
	}
}

func TestExpiredTokenRejected(t *testing.T) {
	issuer := NewTokenIssuer("s", -time.Minute, time.Hour) // already expired
	tok, _ := issuer.IssueAccessToken(uuid.New(), uuid.New(), "x@y.com")
	if _, err := issuer.ParseAccessToken(tok); err == nil {
		t.Error("expired token was accepted")
	}
}

func TestRefreshTokenHashing(t *testing.T) {
	tok, hash, err := GenerateRefreshToken()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if tok == hash {
		t.Fatal("stored hash equals raw token")
	}
	if HashRefreshToken(tok) != hash {
		t.Error("hash is not deterministic")
	}
}

func TestPrincipalCan(t *testing.T) {
	p := &Principal{Permissions: map[string]bool{"worker:read": true}}
	if !p.Can("worker:read") {
		t.Error("expected permission granted")
	}
	if p.Can("worker:write") {
		t.Error("unexpected permission granted")
	}
	var nilP *Principal
	if nilP.Can("anything") {
		t.Error("nil principal should grant nothing")
	}
}
