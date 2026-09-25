// Package service contains the auth domain business logic: registration,
// login, token issuing/verification, refresh rotation, and logout.
package service

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"auth/internal/model"
	"auth/internal/password"
	"auth/internal/repository"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserAlreadyExists  = errors.New("user already exists")
	ErrInvalidToken       = errors.New("invalid token")
	ErrInvalidInput       = errors.New("invalid input")
)

const (
	tokenTypeAccess  = "access"
	tokenTypeRefresh = "refresh"
	passwordMinLen   = 8
)

type Service struct {
	users      repository.UserRepository
	refreshes  repository.RefreshTokenStore
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
	now        func() time.Time
}

func New(users repository.UserRepository, refreshes repository.RefreshTokenStore, secret string, accessTTL, refreshTTL time.Duration) *Service {
	return &Service{
		users:      users,
		refreshes:  refreshes,
		secret:     []byte(secret),
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
		now:        time.Now,
	}
}

func (s *Service) Register(ctx context.Context, in model.RegisterInput) (*model.TokenPair, error) {
	email := strings.TrimSpace(in.Email)
	if _, err := mail.ParseAddress(email); err != nil {
		return nil, fmt.Errorf("%w: invalid email", ErrInvalidInput)
	}
	if len(in.Password) < passwordMinLen {
		return nil, fmt.Errorf("%w: password must be at least %d characters", ErrInvalidInput, passwordMinLen)
	}

	if _, err := s.users.GetByEmail(ctx, email); err == nil {
		return nil, ErrUserAlreadyExists
	} else if !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}

	salt, err := password.NewSalt()
	if err != nil {
		return nil, err
	}
	uid, err := newID()
	if err != nil {
		return nil, err
	}

	cred := &repository.Credential{
		ID:           uid,
		Email:        email,
		Salt:         salt,
		PasswordHash: password.Hash(in.Password, salt),
		CreatedAt:    s.now().UTC(),
	}
	if err := s.users.Create(ctx, cred); err != nil {
		if errors.Is(err, repository.ErrDuplicate) {
			return nil, ErrUserAlreadyExists
		}
		return nil, err
	}

	return s.issueTokens(ctx, uid)
}

func (s *Service) Login(ctx context.Context, in model.LoginInput) (*model.TokenPair, error) {
	email := strings.TrimSpace(in.Email)

	cred, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	attempt := password.Hash(in.Password, cred.Salt)
	if !hmac.Equal([]byte(attempt), []byte(cred.PasswordHash)) {
		return nil, ErrInvalidCredentials
	}
	return s.issueTokens(ctx, cred.ID)
}

func (s *Service) Refresh(ctx context.Context, refreshToken string) (*model.TokenPair, error) {
	hash := refreshTokenHash(refreshToken)
	uid, expiresAt, ok := s.refreshes.Get(ctx, hash)
	if !ok || expiresAt.Before(s.now()) {
		return nil, ErrInvalidToken
	}

	s.refreshes.Delete(ctx, hash)
	return s.issueTokens(ctx, uid)
}

func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	s.refreshes.Delete(ctx, refreshTokenHash(refreshToken))
	return nil
}

func (s *Service) Verify(ctx context.Context, accessToken string) (*model.VerifyResult, error) {
	claims, err := s.verifyToken(accessToken, tokenTypeAccess)
	if err != nil {
		return nil, err
	}

	cred, err := s.users.GetByID(ctx, claims.UserID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrInvalidToken
		}
		return nil, err
	}
	return &model.VerifyResult{UserID: cred.ID, Email: cred.Email}, nil
}

func (s *Service) issueTokens(ctx context.Context, uid string) (*model.TokenPair, error) {
	access, err := s.signToken(uid, tokenTypeAccess, s.accessTTL)
	if err != nil {
		return nil, err
	}
	refresh, err := s.signToken(uid, tokenTypeRefresh, s.refreshTTL)
	if err != nil {
		return nil, err
	}
	if err := s.refreshes.Store(ctx, refreshTokenHash(refresh), uid, s.now().Add(s.refreshTTL)); err != nil {
		return nil, err
	}
	return &model.TokenPair{
		UserID:       uid,
		AccessToken:  access,
		RefreshToken: refresh,
		TokenType:    "Bearer",
		ExpiresIn:    int64(s.accessTTL.Seconds()),
	}, nil
}

type tokenClaims struct {
	JTI    string `json:"jti"`
	UserID string `json:"uid"`
	Type   string `json:"typ"`
	Exp    int64  `json:"exp"`
}

func (s *Service) signToken(uid, typ string, ttl time.Duration) (string, error) {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))

	jti, err := newID()
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(tokenClaims{JTI: jti, UserID: uid, Type: typ, Exp: s.now().Add(ttl).Unix()})
	if err != nil {
		return "", err
	}
	body := header + "." + base64.RawURLEncoding.EncodeToString(payload)
	return body + "." + s.sign(body), nil
}

func (s *Service) verifyToken(token, wantType string) (*tokenClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, ErrInvalidToken
	}

	body := parts[0] + "." + parts[1]
	if subtle.ConstantTimeCompare([]byte(s.sign(body)), []byte(parts[2])) != 1 {
		return nil, ErrInvalidToken
	}

	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrInvalidToken
	}
	var claims tokenClaims
	if err := json.Unmarshal(raw, &claims); err != nil {
		return nil, ErrInvalidToken
	}
	if claims.Type != wantType || claims.UserID == "" {
		return nil, ErrInvalidToken
	}
	if claims.Exp < s.now().Unix() {
		return nil, ErrInvalidToken
	}
	return &claims, nil
}

func (s *Service) sign(body string) string {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(body))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func refreshTokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func newID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
