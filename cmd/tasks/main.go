package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"plans-tracker-backend/internal/platform"
	"plans-tracker-backend/internal/tasks"
	"plans-tracker-backend/internal/tasks/httpapi"
	"plans-tracker-backend/internal/tasks/postgres"
)

func main() {
	if err := platform.Run("tasks", func(mux *http.ServeMux, pool *pgxpool.Pool) {
		repository := postgres.NewRepository(pool)
		service := tasks.NewService(repository)
		userID := os.Getenv("DEV_USER_ID")
		httpapi.NewHandler(service, userID).Register(mux)
	}); err != nil {
		slog.Error("service stopped", "error", err)
		os.Exit(1)
	}
}
