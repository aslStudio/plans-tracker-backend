package platform

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type RegisterRoutes func(*http.ServeMux, *pgxpool.Pool)

func Run(name string, register RegisterRoutes) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("configure postgres: %w", err)
	}
	if cfg.DatabasePass != "" {
		poolConfig.ConnConfig.Password = cfg.DatabasePass
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return fmt.Errorf("connect postgres: %w", err)
	}
	defer pool.Close()

	redisClient := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr, Password: cfg.RedisPass})
	defer redisClient.Close()

	mux := newHandler(name, pool.Ping, func(ctx context.Context) error {
		return redisClient.Ping(ctx).Err()
	})
	if register != nil {
		register(mux, pool)
	}

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serveErr := make(chan error, 1)
	go func() { serveErr <- server.ListenAndServe() }()
	slog.Info("service listening", "service", name, "port", cfg.Port)

	select {
	case err := <-serveErr:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve http: %w", err)
		}
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown http: %w", err)
		}
	}
	return nil
}

func newHandler(name string, postgresPing, redisPing func(context.Context) error) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, "{\"service\":%q,\"status\":\"ok\"}\n", name)
	})
	mux.HandleFunc("GET /health/ready", func(w http.ResponseWriter, r *http.Request) {
		checkCtx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := postgresPing(checkCtx); err != nil {
			slog.Warn("postgres unavailable", "service", name, "error", err)
			http.Error(w, "dependency unavailable", http.StatusServiceUnavailable)
			return
		}
		if err := redisPing(checkCtx); err != nil {
			slog.Warn("redis unavailable", "service", name, "error", err)
			http.Error(w, "dependency unavailable", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, "{\"service\":%q,\"status\":\"ready\"}\n", name)
	})
	return mux
}
