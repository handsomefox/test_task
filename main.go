package main

import (
	"os"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"golang.org/x/exp/slog"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		AddSource: false,
		Level:     slog.LevelDebug,
	}))

	slog.SetDefault(logger)

	if err := godotenv.Load(); err != nil {
		slog.Error("Failed to load environment", "error", err)
		os.Exit(1)
	}

	db, err := NewPostgreSQLDatabase()
	if err != nil {
		slog.Error("Failed to init the database", "error", err)
		os.Exit(1)
	}

	server := NewAPIServer(db, os.Getenv("APP_HOST")+":"+os.Getenv("APP_PORT"))
	if err := server.Run(); err != nil {
		slog.Error("Server run error", "error", err)
		os.Exit(1)
	}
}
