package accounts_test

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"plans-tracker-backend/internal/accounts"
	accountsh "plans-tracker-backend/internal/accounts/httpapi"
	accountspg "plans-tracker-backend/internal/accounts/postgres"
	"plans-tracker-backend/internal/auth"
)

func TestAppleSessionLifecycleWithPostgres(t *testing.T) {
	urls := []string{os.Getenv("ACCOUNTS_TEST_DATABASE_URL"), os.Getenv("TASKS_TEST_DATABASE_URL"), os.Getenv("HABITS_TEST_DATABASE_URL"), os.Getenv("CALENDAR_TEST_DATABASE_URL")}
	for _, value := range urls {
		if value == "" {
			t.Skip("four test database URLs are required")
		}
	}
	ctx := context.Background()
	pools := make([]*pgxpool.Pool, 4)
	for i, value := range urls {
		pool, err := pgxpool.New(ctx, value)
		if err != nil {
			t.Fatal(err)
		}
		pools[i] = pool
		defer pool.Close()
	}
	appleSigningKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	clientKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	subject := fmt.Sprintf("integration-%d", time.Now().UnixNano())
	var nonce string
	var revokeCalls atomic.Int32
	var fake *httptest.Server
	fake = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth/keys":
			json.NewEncoder(w).Encode(map[string]any{"keys": []any{map[string]any{
				"kid": "apple-test", "kty": "RSA", "alg": "RS256", "use": "sig",
				"n": base64.RawURLEncoding.EncodeToString(appleSigningKey.N.Bytes()),
				"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(appleSigningKey.E)).Bytes()),
			}}})
		case "/auth/token":
			if err := r.ParseForm(); err != nil {
				t.Error(err)
				w.WriteHeader(400)
				return
			}
			if r.Form.Get("client_id") != "app.planstracker" || !verifyClientSecret(r.Form.Get("client_secret"), &clientKey.PublicKey) {
				t.Error("invalid client secret")
				w.WriteHeader(400)
				return
			}
			if r.Form.Get("grant_type") == "authorization_code" && r.Form.Get("code") != "one-time-code" {
				w.WriteHeader(400)
				return
			}
			if r.Form.Get("grant_type") == "refresh_token" && r.Form.Get("refresh_token") != "apple-refresh" {
				w.WriteHeader(400)
				return
			}
			json.NewEncoder(w).Encode(map[string]string{"id_token": signedAppleToken(t, appleSigningKey, subject, nonce), "refresh_token": "apple-refresh", "access_token": "apple-access"})
		case "/auth/revoke":
			if err := r.ParseForm(); err != nil || r.Form.Get("token") != "apple-refresh" {
				t.Error("invalid revoke request")
				w.WriteHeader(400)
				return
			}
			revokeCalls.Add(1)
			w.WriteHeader(200)
		default:
			w.WriteHeader(404)
		}
	}))
	defer fake.Close()
	repo := accountspg.Repository{Accounts: pools[0], Tasks: pools[1], Habits: pools[2], Calendar: pools[3]}
	service := accounts.Service{Repo: repo, Apple: &accounts.AppleClient{ClientID: "app.planstracker", TeamID: "X2HSSNMWCF", KeyID: "F2DKUSNVKY", PrivateKey: clientKey, BaseURL: fake.URL, HTTP: fake.Client()}, EncryptionKey: make([]byte, 32)}
	challenge, err := service.NewChallenge(ctx)
	if err != nil {
		t.Fatal(err)
	}
	nonce = challenge.Nonce
	identityToken := signedAppleToken(t, appleSigningKey, subject, nonce)
	if _, err := service.Apple.VerifyIdentity(ctx, identityToken, "wrong-nonce"); err != accounts.ErrInvalidCredentials {
		t.Fatalf("wrong nonce accepted: %v", err)
	}
	parts := strings.Split(identityToken, ".")
	firstSignatureChar := "A"
	if strings.HasPrefix(parts[2], "A") {
		firstSignatureChar = "B"
	}
	parts[2] = firstSignatureChar + parts[2][1:]
	tampered := strings.Join(parts, ".")
	if _, err := service.Apple.VerifyIdentity(ctx, tampered, nonce); err != accounts.ErrInvalidCredentials {
		t.Fatalf("tampered signature accepted: %v", err)
	}
	first, err := service.SignIn(ctx, identityToken, "one-time-code", nonce, "Test User")
	if err != nil {
		t.Fatal(err)
	}
	defer pools[0].Exec(ctx, `DELETE FROM users WHERE id=$1`, first.User.ID)
	if first.User.ID == "" || first.User.DisplayName == nil || *first.User.DisplayName != "Test User" {
		t.Fatalf("unexpected user: %+v", first.User)
	}
	if _, err := service.SignIn(ctx, identityToken, "one-time-code", nonce, ""); err != accounts.ErrUnauthorized {
		t.Fatalf("reused nonce: %v", err)
	}
	user, err := service.Authenticate(ctx, first.AccessToken, false)
	if err != nil || user.ID != first.User.ID {
		t.Fatalf("access lookup: %v", err)
	}
	accountsMux := http.NewServeMux()
	accountsh.Handler{Service: service, InternalSecret: "integration-secret"}.Register(accountsMux)
	accountsServer := httptest.NewServer(accountsMux)
	defer accountsServer.Close()
	protected := (auth.Middleware{IntrospectURL: accountsServer.URL + "/internal/auth/introspect", InternalSecret: "integration-secret"}).Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth.UserID(r.Context()) != first.User.ID {
			t.Error("wrong user in protected request")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	checkAccess := func(token string, want int) {
		r := httptest.NewRequest(http.MethodGet, "/v1/tasks", nil)
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		protected.ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("protected request: got %d, want %d", w.Code, want)
		}
	}
	checkAccess(first.AccessToken, http.StatusNoContent)
	rotated, err := service.Refresh(ctx, first.RefreshToken)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Refresh(ctx, first.RefreshToken); err != accounts.ErrUnauthorized {
		t.Fatalf("old refresh accepted: %v", err)
	}
	if _, err := service.Authenticate(ctx, first.AccessToken, false); err != accounts.ErrUnauthorized {
		t.Fatalf("old access accepted: %v", err)
	}
	if err := service.Logout(ctx, rotated.AccessToken); err != nil {
		t.Fatal(err)
	}
	checkAccess(rotated.AccessToken, http.StatusUnauthorized)
	if _, err := service.Authenticate(ctx, rotated.AccessToken, false); err != accounts.ErrUnauthorized {
		t.Fatalf("revoked access accepted: %v", err)
	}
	challenge, err = service.NewChallenge(ctx)
	if err != nil {
		t.Fatal(err)
	}
	nonce = challenge.Nonce
	second, err := service.SignIn(ctx, signedAppleToken(t, appleSigningKey, subject, nonce), "one-time-code", nonce, "")
	if err != nil {
		t.Fatal(err)
	}
	if second.User.ID != first.User.ID {
		t.Fatal("same Apple subject created a second user")
	}
	consentNotification := signedAppleNotification(t, appleSigningKey, subject, "consent-revoked", "notice-consent-"+subject)
	if err := service.HandleAppleNotification(ctx, consentNotification); err != nil {
		t.Fatal(err)
	}
	if err := service.HandleAppleNotification(ctx, consentNotification); err != nil {
		t.Fatal("duplicate notification: ", err)
	}
	if _, err := service.Authenticate(ctx, second.AccessToken, false); err != accounts.ErrUnauthorized {
		t.Fatalf("revoked Apple consent left session active: %v", err)
	}
	challenge, err = service.NewChallenge(ctx)
	if err != nil {
		t.Fatal(err)
	}
	nonce = challenge.Nonce
	second, err = service.SignIn(ctx, signedAppleToken(t, appleSigningKey, subject, nonce), "one-time-code", nonce, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.HandleAppleNotification(ctx, signedAppleNotification(t, appleSigningKey, subject, "email-disabled", "notice-email-"+subject)); err != nil {
		t.Fatal(err)
	}
	if _, err := pools[1].Exec(ctx, `SELECT ensure_task_defaults($1)`, second.User.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := pools[1].Exec(ctx, `INSERT INTO tasks (user_id,title) VALUES ($1,'private task')`, second.User.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := pools[2].Exec(ctx, `INSERT INTO habits (user_id,title,repeat_rule) VALUES ($1,'private habit','{"type":"daily"}')`, second.User.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := pools[3].Exec(ctx, `INSERT INTO calendar_events (user_id,title,event_date,start_minute,duration_minutes,color) VALUES ($1,'private event',CURRENT_DATE,540,60,'#fff')`, second.User.ID); err != nil {
		t.Fatal(err)
	}
	if err := service.DeleteAccount(ctx, second.AccessToken); err != nil {
		t.Fatal(err)
	}
	if revokeCalls.Load() != 1 {
		t.Fatal("Apple refresh token was not revoked")
	}
	for i, table := range []string{"users", "tasks", "habits", "calendar_events"} {
		var count int
		column := "user_id"
		if i == 0 {
			column = "id"
		}
		if err := pools[i].QueryRow(ctx, `SELECT count(*) FROM `+table+` WHERE `+column+`=$1`, second.User.ID).Scan(&count); err != nil || count != 0 {
			t.Fatalf("%s not deleted: count=%d err=%v", table, count, err)
		}
	}
	subject = fmt.Sprintf("integration-deleted-%d", time.Now().UnixNano())
	challenge, err = service.NewChallenge(ctx)
	if err != nil {
		t.Fatal(err)
	}
	nonce = challenge.Nonce
	deletedByApple, err := service.SignIn(ctx, signedAppleToken(t, appleSigningKey, subject, nonce), "one-time-code", nonce, "")
	if err != nil {
		t.Fatal(err)
	}
	defer pools[0].Exec(ctx, `DELETE FROM users WHERE id=$1`, deletedByApple.User.ID)
	if _, err := pools[3].Exec(ctx, `INSERT INTO calendar_events (user_id,title,event_date,start_minute,duration_minutes,color) VALUES ($1,'deleted event',CURRENT_DATE,540,60,'#fff')`, deletedByApple.User.ID); err != nil {
		t.Fatal(err)
	}
	if err := service.HandleAppleNotification(ctx, signedAppleNotification(t, appleSigningKey, subject, "account-deleted", "notice-deleted-"+subject)); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Authenticate(ctx, deletedByApple.AccessToken, false); err != accounts.ErrUnauthorized {
		t.Fatalf("deleted Apple account still authenticated: %v", err)
	}
	var remaining int
	if err := pools[3].QueryRow(ctx, `SELECT count(*) FROM calendar_events WHERE user_id=$1`, deletedByApple.User.ID).Scan(&remaining); err != nil || remaining != 0 {
		t.Fatalf("Apple account deletion left event: %d %v", remaining, err)
	}
}

func signedAppleToken(t *testing.T, key *rsa.PrivateKey, subject, nonce string) string {
	t.Helper()
	header, _ := json.Marshal(map[string]any{"alg": "RS256", "kid": "apple-test"})
	payload, _ := json.Marshal(map[string]any{"iss": "https://appleid.apple.com", "aud": "app.planstracker", "sub": subject, "nonce": nonce, "iat": time.Now().Unix(), "exp": time.Now().Add(time.Hour).Unix(), "email": "test@example.invalid"})
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	digest := sha256.Sum256([]byte(unsigned))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func signedAppleNotification(t *testing.T, key *rsa.PrivateKey, subject, kind, id string) string {
	t.Helper()
	header, _ := json.Marshal(map[string]any{"alg": "RS256", "kid": "apple-test"})
	payload, _ := json.Marshal(map[string]any{"iss": "https://appleid.apple.com", "aud": "app.planstracker", "iat": time.Now().Unix(), "jti": id, "events": map[string]any{"type": kind, "sub": subject, "event_time": time.Now().Unix()}})
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	digest := sha256.Sum256([]byte(unsigned))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(signature)
}
func verifyClientSecret(token string, key *ecdsa.PublicKey) bool {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return false
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(sig) != 64 {
		return false
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if !ecdsa.Verify(key, digest[:], new(big.Int).SetBytes(sig[:32]), new(big.Int).SetBytes(sig[32:])) {
		return false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return false
	}
	var claims struct {
		Issuer   string `json:"iss"`
		Subject  string `json:"sub"`
		Audience string `json:"aud"`
	}
	return json.Unmarshal(payload, &claims) == nil && claims.Issuer == "X2HSSNMWCF" && claims.Subject == "app.planstracker" && claims.Audience == "https://appleid.apple.com"
}
