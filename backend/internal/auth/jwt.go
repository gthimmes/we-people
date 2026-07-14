package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Claims are the JWT claims carried by an access token.
type Claims struct {
	OrgID uuid.UUID `json:"org_id"`
	Email string    `json:"email"`
	jwt.RegisteredClaims
}

// TokenIssuer mints and verifies access tokens and generates refresh tokens.
type TokenIssuer struct {
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
}

// NewTokenIssuer creates a TokenIssuer.
func NewTokenIssuer(secret string, accessTTL, refreshTTL time.Duration) *TokenIssuer {
	return &TokenIssuer{secret: []byte(secret), accessTTL: accessTTL, refreshTTL: refreshTTL}
}

// AccessTTL exposes the configured access-token lifetime.
func (t *TokenIssuer) AccessTTL() time.Duration { return t.accessTTL }

// RefreshTTL exposes the configured refresh-token lifetime.
func (t *TokenIssuer) RefreshTTL() time.Duration { return t.refreshTTL }

// IssueAccessToken mints a signed JWT for the given user.
func (t *TokenIssuer) IssueAccessToken(userID, orgID uuid.UUID, email string) (string, error) {
	now := time.Now()
	claims := Claims{
		OrgID: orgID,
		Email: email,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(t.accessTTL)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(t.secret)
}

// ParseAccessToken verifies a token's signature and expiry and returns its claims.
func (t *TokenIssuer) ParseAccessToken(tokenStr string) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(tokenStr, claims, func(tok *jwt.Token) (any, error) {
		if _, ok := tok.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", tok.Header["alg"])
		}
		return t.secret, nil
	})
	if err != nil {
		return nil, err
	}
	return claims, nil
}

// GenerateRefreshToken returns a new opaque refresh token and its SHA-256 hash.
// Only the hash is ever stored server-side.
func GenerateRefreshToken() (token, hash string, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}
	token = base64.RawURLEncoding.EncodeToString(buf)
	return token, HashRefreshToken(token), nil
}

// HashRefreshToken returns the SHA-256 hex hash of a refresh token.
func HashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
