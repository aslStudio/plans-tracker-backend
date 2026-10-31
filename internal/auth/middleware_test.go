package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMiddlewareRequiresBearerAndUsesIntrospection(t *testing.T) {
	introspection := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Internal-Auth") != "secret" {
			t.Fatal("missing internal secret")
		}
		if r.Header.Get("Authorization") != "Bearer valid" {
			http.Error(w, "unauthorized", 401)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"user_id":"user-1"}`))
	}))
	defer introspection.Close()
	wrapped := (Middleware{IntrospectURL: introspection.URL, InternalSecret: "secret"}).Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if UserID(r.Context()) != "user-1" || AccessToken(r.Context()) != "valid" {
			t.Fatal("identity missing from context")
		}
		w.WriteHeader(204)
	}))
	for _, tc := range []struct {
		token  string
		status int
	}{{"", 401}, {"Bearer invalid", 401}, {"Bearer valid", 204}} {
		r := httptest.NewRequest("GET", "/v1/tasks", nil)
		if tc.token != "" {
			r.Header.Set("Authorization", tc.token)
		}
		w := httptest.NewRecorder()
		wrapped.ServeHTTP(w, r)
		if w.Code != tc.status {
			t.Fatalf("%q: got %d, want %d", tc.token, w.Code, tc.status)
		}
	}
}
