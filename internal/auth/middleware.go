package auth

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

type Middleware struct {
	IntrospectURL  string
	InternalSecret string
	DevUserID      string
	AllowDevUser   bool
	Client         *http.Client
}

func (m Middleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/v1/") {
			next.ServeHTTP(w, r)
			return
		}
		header := r.Header.Get("Authorization")
		if header == "" && m.AllowDevUser && m.DevUserID != "" {
			next.ServeHTTP(w, r.WithContext(WithIdentity(r.Context(), m.DevUserID, "")))
			return
		}
		parts := strings.Fields(header)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if m.IntrospectURL == "" || m.InternalSecret == "" {
			http.Error(w, "authentication unavailable", http.StatusServiceUnavailable)
			return
		}
		request, err := http.NewRequestWithContext(r.Context(), http.MethodPost, m.IntrospectURL, nil)
		if err != nil {
			http.Error(w, "authentication unavailable", http.StatusServiceUnavailable)
			return
		}
		request.Header.Set("Authorization", "Bearer "+parts[1])
		request.Header.Set("X-Internal-Auth", m.InternalSecret)
		client := m.Client
		if client == nil {
			client = &http.Client{Timeout: 2 * time.Second}
		}
		response, err := client.Do(request)
		if err != nil {
			http.Error(w, "authentication unavailable", http.StatusServiceUnavailable)
			return
		}
		defer response.Body.Close()
		if response.StatusCode == http.StatusUnauthorized {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if response.StatusCode != http.StatusOK {
			http.Error(w, "authentication unavailable", http.StatusServiceUnavailable)
			return
		}
		var result struct {
			UserID string `json:"user_id"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, response.Body, 1024)).Decode(&result); err != nil || result.UserID == "" {
			http.Error(w, "authentication unavailable", http.StatusServiceUnavailable)
			return
		}
		next.ServeHTTP(w, r.WithContext(WithIdentity(r.Context(), result.UserID, parts[1])))
	})
}
