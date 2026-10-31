package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"plans-tracker-backend/internal/calendar"
	"plans-tracker-backend/internal/calendar/httpapi"
	"plans-tracker-backend/internal/calendar/postgres"
	"plans-tracker-backend/internal/platform"
)

func main() {
	if err := platform.Run("calendar", func(mux *http.ServeMux, pool *pgxpool.Pool) {
		httpapi.Handler{Service: calendar.Service{
			Repository: postgres.Repository{Pool: pool},
			Links:      calendar.HTTPLinks{TasksURL: os.Getenv("TASKS_URL"), HabitsURL: os.Getenv("HABITS_URL")},
		}, UserID: os.Getenv("DEV_USER_ID")}.Register(mux)
	}); err != nil {
		slog.Error("service stopped", "error", err)
		os.Exit(1)
	}
}
