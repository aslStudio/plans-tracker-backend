package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"plans-tracker-backend/internal/accounts"
	"plans-tracker-backend/internal/accounts/httpapi"
	accountspg "plans-tracker-backend/internal/accounts/postgres"
	"plans-tracker-backend/internal/platform"
)

func additionalPool(value string) (*pgxpool.Pool, error) {
	url := os.Getenv(value)
	if url == "" {
		return nil, fmt.Errorf("%s is required", value)
	}
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, err
	}
	if password := os.Getenv("DATABASE_PASSWORD"); password != "" {
		config.ConnConfig.Password = password
	}
	return pgxpool.NewWithConfig(context.Background(), config)
}

func run() error {
	teamID, keyID, clientID := os.Getenv("APPLE_TEAM_ID"), os.Getenv("APPLE_KEY_ID"), os.Getenv("APPLE_CLIENT_ID")
	if len(teamID) != 10 || len(keyID) != 10 || clientID == "" {
		return fmt.Errorf("APPLE_TEAM_ID, APPLE_KEY_ID and APPLE_CLIENT_ID are required")
	}
	secret := os.Getenv("AUTH_INTERNAL_SECRET")
	if len(secret) < 32 {
		return fmt.Errorf("AUTH_INTERNAL_SECRET must contain at least 32 characters")
	}
	keyData, err := os.ReadFile(os.Getenv("APPLE_PRIVATE_KEY_FILE"))
	if err != nil {
		return fmt.Errorf("read Apple key: %w", err)
	}
	privateKey, err := accounts.ParseApplePrivateKey(keyData)
	if err != nil {
		return err
	}
	keyData = nil
	encryptionKey, err := base64.StdEncoding.DecodeString(os.Getenv("APPLE_TOKEN_ENCRYPTION_KEY"))
	if err != nil || len(encryptionKey) != 32 {
		return fmt.Errorf("APPLE_TOKEN_ENCRYPTION_KEY must be base64 of 32 bytes")
	}
	tasks, err := additionalPool("TASKS_DATABASE_URL")
	if err != nil {
		return err
	}
	defer tasks.Close()
	habits, err := additionalPool("HABITS_DATABASE_URL")
	if err != nil {
		return err
	}
	defer habits.Close()
	calendar, err := additionalPool("CALENDAR_DATABASE_URL")
	if err != nil {
		return err
	}
	defer calendar.Close()
	apple := &accounts.AppleClient{ClientID: clientID, TeamID: teamID, KeyID: keyID, PrivateKey: privateKey}
	return platform.Run("accounts", func(mux *http.ServeMux, pool *pgxpool.Pool) {
		httpapi.Handler{Service: accounts.Service{
			Repo:  accountspg.Repository{Accounts: pool, Tasks: tasks, Habits: habits, Calendar: calendar},
			Apple: apple, EncryptionKey: encryptionKey,
		}, InternalSecret: secret}.Register(mux)
	})
}

func main() {
	if err := run(); err != nil {
		slog.Error("service stopped", "error", err)
		os.Exit(1)
	}
}
