package accounts

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"strings"
	"time"
)

var ErrInvalidInput = errors.New("invalid input")
var ErrUnauthorized = errors.New("unauthorized")
var ErrDeletionUnavailable = errors.New("account deletion unavailable")

const accessLifetime = 15 * time.Minute
const refreshLifetime = 30 * 24 * time.Hour

type User struct {
	ID          string  `json:"id"`
	Email       *string `json:"email"`
	DisplayName *string `json:"display_name"`
}

type Session struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	User         User   `json:"user"`
}

type Challenge struct {
	Nonce     string    `json:"nonce"`
	ExpiresAt time.Time `json:"expires_at"`
}

type IdentityRecord struct {
	User             User
	AppleSub         string
	EncryptedRefresh []byte
	LastVerifiedAt   *time.Time
	DeletionPending  bool
}

type Repository interface {
	SaveChallenge(context.Context, []byte, time.Time) error
	ConsumeChallenge(context.Context, []byte) (bool, error)
	UpsertAppleIdentity(context.Context, AppleIdentity, *string, []byte) (User, error)
	NewSession(context.Context, string, []byte, []byte, time.Time, time.Time) error
	LookupRefresh(context.Context, []byte) (IdentityRecord, error)
	RotateSession(context.Context, []byte, []byte, []byte, time.Time, time.Time) (bool, error)
	TouchAppleVerification(context.Context, string) error
	RevokeUserSessions(context.Context, string) error
	UserByAccess(context.Context, []byte, bool) (User, error)
	RevokeSession(context.Context, []byte) (bool, error)
	MarkDeleting(context.Context, string) error
	DeleteUserData(context.Context, string) error
	DeleteUser(context.Context, string) error
	AppleRefreshForUser(context.Context, string) ([]byte, error)
	NotificationSeen(context.Context, string) (bool, error)
	UserIDByAppleSub(context.Context, string) (string, error)
	SetEmailForwarding(context.Context, string, bool, *string) error
	ClearAppleRefresh(context.Context, string) error
	RecordNotification(context.Context, string) error
}

type Service struct {
	Repo          Repository
	Apple         *AppleClient
	EncryptionKey []byte
}

func randomToken() (string, error) {
	data := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, data); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}
func tokenHash(token string) []byte {
	hash := sha256.Sum256([]byte(token))
	return hash[:]
}
func validOpaqueToken(token string) bool {
	if len(token) != 43 {
		return false
	}
	data, err := base64.RawURLEncoding.DecodeString(token)
	return err == nil && len(data) == 32
}
func (s Service) encrypt(value string) ([]byte, error) {
	if len(s.EncryptionKey) != 32 {
		return nil, ErrProviderUnavailable
	}
	block, err := aes.NewCipher(s.EncryptionKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, []byte(value), nil), nil
}
func (s Service) decrypt(value []byte) (string, error) {
	if len(s.EncryptionKey) != 32 {
		return "", ErrProviderUnavailable
	}
	block, err := aes.NewCipher(s.EncryptionKey)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(value) < gcm.NonceSize() {
		return "", ErrProviderUnavailable
	}
	plain, err := gcm.Open(nil, value[:gcm.NonceSize()], value[gcm.NonceSize():], nil)
	if err != nil {
		return "", ErrProviderUnavailable
	}
	return string(plain), nil
}

func (s Service) NewChallenge(ctx context.Context) (Challenge, error) {
	nonce, err := randomToken()
	if err != nil {
		return Challenge{}, err
	}
	expires := time.Now().Add(5 * time.Minute)
	if err := s.Repo.SaveChallenge(ctx, tokenHash(nonce), expires); err != nil {
		return Challenge{}, err
	}
	return Challenge{Nonce: nonce, ExpiresAt: expires}, nil
}

func (s Service) SignIn(ctx context.Context, identityToken, code, nonce, displayName string) (Session, error) {
	if s.Apple == nil || len(s.EncryptionKey) != 32 {
		return Session{}, ErrProviderUnavailable
	}
	if identityToken == "" || code == "" || !validOpaqueToken(nonce) || len(displayName) > 200 {
		return Session{}, ErrInvalidInput
	}
	consumed, err := s.Repo.ConsumeChallenge(ctx, tokenHash(nonce))
	if err != nil {
		return Session{}, err
	}
	if !consumed {
		return Session{}, ErrUnauthorized
	}
	identity, err := s.Apple.VerifyIdentity(ctx, identityToken, nonce)
	if err != nil {
		return Session{}, err
	}
	tokens, err := s.Apple.ExchangeCode(ctx, code)
	if err != nil {
		return Session{}, err
	}
	exchanged, err := s.Apple.VerifyIdentity(ctx, tokens.IdentityToken, "")
	if err != nil {
		return Session{}, err
	}
	if exchanged.Subject != identity.Subject {
		return Session{}, ErrInvalidCredentials
	}
	if tokens.RefreshToken == "" {
		return Session{}, ErrProviderUnavailable
	}
	encrypted, err := s.encrypt(tokens.RefreshToken)
	if err != nil {
		return Session{}, err
	}
	var name *string
	if trimmed := strings.TrimSpace(displayName); trimmed != "" {
		name = &trimmed
	}
	user, err := s.Repo.UpsertAppleIdentity(ctx, identity, name, encrypted)
	if err != nil {
		return Session{}, err
	}
	return s.issue(ctx, user)
}

func (s Service) issue(ctx context.Context, user User) (Session, error) {
	access, err := randomToken()
	if err != nil {
		return Session{}, err
	}
	refresh, err := randomToken()
	if err != nil {
		return Session{}, err
	}
	now := time.Now()
	if err := s.Repo.NewSession(ctx, user.ID, tokenHash(access), tokenHash(refresh), now.Add(accessLifetime), now.Add(refreshLifetime)); err != nil {
		return Session{}, err
	}
	return Session{AccessToken: access, RefreshToken: refresh, TokenType: "Bearer", ExpiresIn: int(accessLifetime.Seconds()), User: user}, nil
}

func (s Service) Refresh(ctx context.Context, oldToken string) (Session, error) {
	if !validOpaqueToken(oldToken) {
		return Session{}, ErrUnauthorized
	}
	record, err := s.Repo.LookupRefresh(ctx, tokenHash(oldToken))
	if err != nil {
		return Session{}, err
	}
	if !record.DeletionPending && (record.LastVerifiedAt == nil || time.Since(*record.LastVerifiedAt) >= 24*time.Hour) {
		if s.Apple == nil {
			return Session{}, ErrProviderUnavailable
		}
		appleRefresh, err := s.decrypt(record.EncryptedRefresh)
		if err != nil {
			return Session{}, err
		}
		identity, err := s.Apple.ValidateRefresh(ctx, appleRefresh)
		if errors.Is(err, ErrInvalidCredentials) || (err == nil && identity.Subject != record.AppleSub) {
			_ = s.Repo.RevokeUserSessions(ctx, record.User.ID)
			return Session{}, ErrUnauthorized
		}
		if err != nil {
			return Session{}, err
		}
		if err := s.Repo.TouchAppleVerification(ctx, record.AppleSub); err != nil {
			return Session{}, err
		}
	}
	access, err := randomToken()
	if err != nil {
		return Session{}, err
	}
	refresh, err := randomToken()
	if err != nil {
		return Session{}, err
	}
	now := time.Now()
	rotated, err := s.Repo.RotateSession(ctx, tokenHash(oldToken), tokenHash(access), tokenHash(refresh), now.Add(accessLifetime), now.Add(refreshLifetime))
	if err != nil {
		return Session{}, err
	}
	if !rotated {
		return Session{}, ErrUnauthorized
	}
	return Session{AccessToken: access, RefreshToken: refresh, TokenType: "Bearer", ExpiresIn: int(accessLifetime.Seconds()), User: record.User}, nil
}

func (s Service) Authenticate(ctx context.Context, token string, allowDeleting bool) (User, error) {
	if !validOpaqueToken(token) {
		return User{}, ErrUnauthorized
	}
	return s.Repo.UserByAccess(ctx, tokenHash(token), allowDeleting)
}
func (s Service) Logout(ctx context.Context, token string) error {
	if !validOpaqueToken(token) {
		return ErrUnauthorized
	}
	changed, err := s.Repo.RevokeSession(ctx, tokenHash(token))
	if err != nil {
		return err
	}
	if !changed {
		return ErrUnauthorized
	}
	return nil
}
func (s Service) DeleteAccount(ctx context.Context, accessToken string) error {
	user, err := s.Authenticate(ctx, accessToken, true)
	if err != nil {
		return err
	}
	if err := s.Repo.MarkDeleting(ctx, user.ID); err != nil {
		return err
	}
	if err := s.Repo.DeleteUserData(ctx, user.ID); err != nil {
		return ErrDeletionUnavailable
	}
	encrypted, err := s.Repo.AppleRefreshForUser(ctx, user.ID)
	if err != nil {
		return ErrDeletionUnavailable
	}
	if len(encrypted) > 0 {
		refresh, err := s.decrypt(encrypted)
		if err != nil {
			return ErrDeletionUnavailable
		}
		if err := s.Apple.Revoke(ctx, refresh); err != nil {
			return ErrDeletionUnavailable
		}
	}
	if err := s.Repo.DeleteUser(ctx, user.ID); err != nil {
		return ErrDeletionUnavailable
	}
	return nil
}

func (s Service) HandleAppleNotification(ctx context.Context, signedPayload string) error {
	if s.Apple == nil {
		return ErrProviderUnavailable
	}
	event, err := s.Apple.VerifyNotification(ctx, signedPayload)
	if err != nil {
		return err
	}
	seen, err := s.Repo.NotificationSeen(ctx, event.ID)
	if err != nil || seen {
		return err
	}
	userID, err := s.Repo.UserIDByAppleSub(ctx, event.Subject)
	if err != nil {
		return err
	}
	if userID != "" {
		switch event.Type {
		case "email-enabled", "email-disabled":
			if err := s.Repo.SetEmailForwarding(ctx, event.Subject, event.Type == "email-enabled", event.Email); err != nil {
				return err
			}
		case "consent-revoked":
			if err := s.Repo.RevokeUserSessions(ctx, userID); err != nil {
				return err
			}
			if err := s.Repo.ClearAppleRefresh(ctx, event.Subject); err != nil {
				return err
			}
		case "account-deleted":
			if err := s.Repo.MarkDeleting(ctx, userID); err != nil {
				return err
			}
			if err := s.Repo.DeleteUserData(ctx, userID); err != nil {
				return ErrDeletionUnavailable
			}
			if err := s.Repo.DeleteUser(ctx, userID); err != nil {
				return ErrDeletionUnavailable
			}
		}
	}
	return s.Repo.RecordNotification(ctx, event.ID)
}
