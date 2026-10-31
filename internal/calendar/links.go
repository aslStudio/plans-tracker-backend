package calendar

import (
	"context"
	"net/http"
	"net/url"
	"plans-tracker-backend/internal/auth"
	"strings"
	"time"
)

type HTTPLinks struct {
	TasksURL, HabitsURL string
	Client              *http.Client
}

func (v HTTPLinks) Validate(ctx context.Context, link Link) error {
	base, resource := v.TasksURL, "tasks"
	if link.Type == "habit" {
		base, resource = v.HabitsURL, "habits"
	}
	if base == "" {
		return ErrDependency
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(base, "/")+"/v1/"+resource+"/"+url.PathEscape(link.ID), nil)
	if err != nil {
		return ErrDependency
	}
	if token := auth.AccessToken(ctx); token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	client := v.Client
	if client == nil {
		client = &http.Client{Timeout: 2 * time.Second}
	}
	response, err := client.Do(request)
	if err != nil {
		return ErrDependency
	}
	defer response.Body.Close()
	switch response.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusNotFound:
		return ErrInvalidLink
	default:
		return ErrDependency
	}
}
