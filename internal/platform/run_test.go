package platform

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthEndpoints(t *testing.T) {
	pingOK := func(context.Context) error { return nil }
	pingFail := func(context.Context) error { return errors.New("unavailable") }

	tests := []struct {
		name       string
		path       string
		postgres   func(context.Context) error
		redis      func(context.Context) error
		wantStatus int
		wantBody   string
	}{
		{"live without dependencies", "/health/live", pingFail, pingFail, http.StatusOK, `"status":"ok"`},
		{"ready with dependencies", "/health/ready", pingOK, pingOK, http.StatusOK, `"status":"ready"`},
		{"postgres unavailable", "/health/ready", pingFail, pingOK, http.StatusServiceUnavailable, "dependency unavailable"},
		{"redis unavailable", "/health/ready", pingOK, pingFail, http.StatusServiceUnavailable, "dependency unavailable"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, tc.path, nil)
			newHandler("tasks", tc.postgres, tc.redis).ServeHTTP(recorder, request)
			if recorder.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, tc.wantStatus)
			}
			if !strings.Contains(recorder.Body.String(), tc.wantBody) {
				t.Fatalf("body = %q, want to contain %q", recorder.Body.String(), tc.wantBody)
			}
		})
	}
}
