package main

import (
	"log/slog"
	"os"

	"plans-tracker-backend/internal/platform"
)

func main() {
	if err := platform.Run("habits", nil); err != nil {
		slog.Error("service stopped", "error", err)
		os.Exit(1)
	}
}
