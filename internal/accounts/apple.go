package accounts

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

var ErrInvalidCredentials = errors.New("invalid credentials")
var ErrProviderUnavailable = errors.New("apple unavailable")

type AppleIdentity struct {
	Subject string
	Email   *string
}

type AppleTokens struct {
	IdentityToken string `json:"id_token"`
	RefreshToken  string `json:"refresh_token"`
	AccessToken   string `json:"access_token"`
}

type appleKey struct {
	KeyID     string `json:"kid"`
	KeyType   string `json:"kty"`
	Algorithm string `json:"alg"`
	Use       string `json:"use"`
	N         string `json:"n"`
	E         string `json:"e"`
}

type AppleClient struct {
	ClientID   string
	TeamID     string
	KeyID      string
	PrivateKey *ecdsa.PrivateKey
	HTTP       *http.Client
	BaseURL    string
	mu         sync.Mutex
	keys       map[string]*rsa.PublicKey
	keysUntil  time.Time
}

func ParseApplePrivateKey(data []byte) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("invalid Apple private key PEM")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	ecdsaKey, ok := key.(*ecdsa.PrivateKey)
	if !ok || ecdsaKey.Curve.Params().BitSize != 256 {
		return nil, errors.New("Apple private key must be P-256")
	}
	return ecdsaKey, nil
}

func (a *AppleClient) client() *http.Client {
	if a.HTTP != nil {
		return a.HTTP
	}
	return &http.Client{Timeout: 5 * time.Second}
}
func (a *AppleClient) base() string {
	if a.BaseURL != "" {
		return strings.TrimRight(a.BaseURL, "/")
	}
	return "https://appleid.apple.com"
}

func (a *AppleClient) publicKeys(ctx context.Context) (map[string]*rsa.PublicKey, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if time.Now().Before(a.keysUntil) && len(a.keys) != 0 {
		return a.keys, nil
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, a.base()+"/auth/keys", nil)
	if err != nil {
		return nil, err
	}
	response, err := a.client().Do(request)
	if err != nil {
		return nil, ErrProviderUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, ErrProviderUnavailable
	}
	var set struct {
		Keys []appleKey `json:"keys"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 64<<10)).Decode(&set); err != nil {
		return nil, ErrProviderUnavailable
	}
	keys := make(map[string]*rsa.PublicKey)
	for _, k := range set.Keys {
		if k.KeyID == "" || k.KeyType != "RSA" || k.Algorithm != "RS256" || k.Use != "sig" {
			continue
		}
		n, errN := base64.RawURLEncoding.DecodeString(k.N)
		e, errE := base64.RawURLEncoding.DecodeString(k.E)
		if errN != nil || errE != nil || len(n) < 256 || len(e) == 0 || len(e) > 4 {
			continue
		}
		exponent := new(big.Int).SetBytes(e).Int64()
		if exponent < 3 || exponent > 1<<31-1 || exponent%2 == 0 {
			continue
		}
		keys[k.KeyID] = &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(exponent)}
	}
	if len(keys) == 0 {
		return nil, ErrProviderUnavailable
	}
	a.keys, a.keysUntil = keys, time.Now().Add(time.Hour)
	return keys, nil
}

func (a *AppleClient) VerifyIdentity(ctx context.Context, token, expectedNonce string) (AppleIdentity, error) {
	payload, err := a.verifiedPayload(ctx, token)
	if err != nil {
		return AppleIdentity{}, err
	}
	var claims struct {
		Issuer   string  `json:"iss"`
		Audience string  `json:"aud"`
		Subject  string  `json:"sub"`
		Expires  int64   `json:"exp"`
		IssuedAt int64   `json:"iat"`
		Nonce    string  `json:"nonce"`
		Email    *string `json:"email"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return AppleIdentity{}, ErrInvalidCredentials
	}
	now := time.Now().Unix()
	if claims.Issuer != "https://appleid.apple.com" || claims.Audience != a.ClientID || claims.Subject == "" ||
		claims.Expires <= now || claims.IssuedAt > now+60 || claims.IssuedAt <= 0 ||
		(expectedNonce != "" && claims.Nonce != expectedNonce) {
		return AppleIdentity{}, ErrInvalidCredentials
	}
	return AppleIdentity{Subject: claims.Subject, Email: claims.Email}, nil
}

func (a *AppleClient) verifiedPayload(ctx context.Context, token string) ([]byte, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 || len(token) > 16384 {
		return nil, ErrInvalidCredentials
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, ErrInvalidCredentials
	}
	var header struct {
		Algorithm string `json:"alg"`
		KeyID     string `json:"kid"`
		Type      string `json:"typ"`
	}
	if err := json.Unmarshal(headerBytes, &header); err != nil || header.Algorithm != "RS256" || header.KeyID == "" {
		return nil, ErrInvalidCredentials
	}
	keys, err := a.publicKeys(ctx)
	if err != nil {
		return nil, err
	}
	key := keys[header.KeyID]
	if key == nil { // Apple may have rotated keys since the last request.
		a.mu.Lock()
		a.keysUntil = time.Time{}
		a.mu.Unlock()
		keys, err = a.publicKeys(ctx)
		if err != nil {
			return nil, err
		}
		key = keys[header.KeyID]
		if key == nil {
			return nil, ErrInvalidCredentials
		}
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, ErrInvalidCredentials
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if rsa.VerifyPKCS1v15(key, crypto.SHA256, digest[:], sig) != nil {
		return nil, ErrInvalidCredentials
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrInvalidCredentials
	}
	return payload, nil
}

type AppleNotification struct {
	ID      string
	Type    string
	Subject string
	Email   *string
}

func (a *AppleClient) VerifyNotification(ctx context.Context, token string) (AppleNotification, error) {
	payload, err := a.verifiedPayload(ctx, token)
	if err != nil {
		return AppleNotification{}, err
	}
	var claims struct {
		Issuer   string `json:"iss"`
		Audience string `json:"aud"`
		IssuedAt int64  `json:"iat"`
		ID       string `json:"jti"`
		Events   struct {
			Type    string  `json:"type"`
			Subject string  `json:"sub"`
			Email   *string `json:"email"`
		} `json:"events"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return AppleNotification{}, ErrInvalidCredentials
	}
	now := time.Now().Unix()
	if claims.Issuer != "https://appleid.apple.com" || claims.Audience != a.ClientID || claims.IssuedAt <= 0 || claims.IssuedAt > now+60 || claims.ID == "" || claims.Events.Subject == "" {
		return AppleNotification{}, ErrInvalidCredentials
	}
	switch claims.Events.Type {
	case "consent-revoked", "account-deleted", "email-enabled", "email-disabled":
	default:
		return AppleNotification{}, ErrInvalidCredentials
	}
	return AppleNotification{ID: claims.ID, Type: claims.Events.Type, Subject: claims.Events.Subject, Email: claims.Events.Email}, nil
}

func (a *AppleClient) clientSecret() (string, error) {
	if a.PrivateKey == nil || a.TeamID == "" || a.KeyID == "" || a.ClientID == "" {
		return "", ErrProviderUnavailable
	}
	now := time.Now().Unix()
	header, _ := json.Marshal(map[string]string{"alg": "ES256", "kid": a.KeyID, "typ": "JWT"})
	claims, _ := json.Marshal(map[string]any{"iss": a.TeamID, "iat": now, "exp": now + 3600, "aud": "https://appleid.apple.com", "sub": a.ClientID})
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(claims)
	digest := sha256.Sum256([]byte(unsigned))
	r, s, err := ecdsa.Sign(rand.Reader, a.PrivateKey, digest[:])
	if err != nil {
		return "", err
	}
	signature := make([]byte, 64)
	r.FillBytes(signature[:32])
	s.FillBytes(signature[32:])
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func (a *AppleClient) postForm(ctx context.Context, endpoint string, values url.Values) (AppleTokens, error) {
	secret, err := a.clientSecret()
	if err != nil {
		return AppleTokens{}, err
	}
	values.Set("client_id", a.ClientID)
	values.Set("client_secret", secret)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, a.base()+endpoint, strings.NewReader(values.Encode()))
	if err != nil {
		return AppleTokens{}, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := a.client().Do(request)
	if err != nil {
		return AppleTokens{}, ErrProviderUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusBadRequest || response.StatusCode == http.StatusUnauthorized {
		return AppleTokens{}, ErrInvalidCredentials
	}
	if response.StatusCode != http.StatusOK {
		return AppleTokens{}, ErrProviderUnavailable
	}
	var tokens AppleTokens
	if err := json.NewDecoder(io.LimitReader(response.Body, 32<<10)).Decode(&tokens); err != nil || tokens.IdentityToken == "" {
		return AppleTokens{}, ErrProviderUnavailable
	}
	return tokens, nil
}

func (a *AppleClient) ExchangeCode(ctx context.Context, code string) (AppleTokens, error) {
	if code == "" {
		return AppleTokens{}, ErrInvalidCredentials
	}
	return a.postForm(ctx, "/auth/token", url.Values{"grant_type": {"authorization_code"}, "code": {code}})
}

func (a *AppleClient) ValidateRefresh(ctx context.Context, refresh string) (AppleIdentity, error) {
	tokens, err := a.postForm(ctx, "/auth/token", url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refresh}})
	if err != nil {
		return AppleIdentity{}, err
	}
	return a.VerifyIdentity(ctx, tokens.IdentityToken, "")
}

func (a *AppleClient) Revoke(ctx context.Context, refresh string) error {
	if refresh == "" {
		return nil
	}
	secret, err := a.clientSecret()
	if err != nil {
		return err
	}
	values := url.Values{"client_id": {a.ClientID}, "client_secret": {secret}, "token": {refresh}, "token_type_hint": {"refresh_token"}}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, a.base()+"/auth/revoke", strings.NewReader(values.Encode()))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := a.client().Do(request)
	if err != nil {
		return ErrProviderUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("apple revocation failed: %w", ErrProviderUnavailable)
	}
	return nil
}
