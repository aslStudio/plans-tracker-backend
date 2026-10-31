package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"plans-tracker-backend/internal/habits"
	"plans-tracker-backend/internal/habits/httpapi"
	"plans-tracker-backend/internal/habits/postgres"
	"plans-tracker-backend/internal/platform"
)

func main() {
	if err := platform.Run("habits", func(mux *http.ServeMux, pool *pgxpool.Pool) {
		repository := postgres.NewRepository(pool)
		service := habits.NewService(repository)
		httpapi.NewHandler(service, os.Getenv("DEV_USER_ID")).Register(mux)
	}); err != nil {
		slog.Error("service stopped", "error", err)
		os.Exit(1)
	}
}
