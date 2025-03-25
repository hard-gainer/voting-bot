package logger

import (
	"log/slog"
	"os"
)

func InitLogger() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	slog.SetDefault(logger)
}
