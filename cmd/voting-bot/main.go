package main

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hard-gainer/voting-bot/internal/config"
	"github.com/hard-gainer/voting-bot/internal/db"
	"github.com/hard-gainer/voting-bot/internal/logger"
	"github.com/hard-gainer/voting-bot/internal/mattermost"
	"github.com/hard-gainer/voting-bot/internal/service"
	"github.com/tarantool/go-tarantool"
)

func main() {
	logger.InitLogger()
	slog.Info("Starting Mattermost voting bot...")

	cfg := config.NewConfig()
	slog.Info("Config loaded",
		"tarantool_addr", cfg.TarantoolAddr,
		"mattermost_url", cfg.MattermostBotURL,
		"mattermost_bot_addr", cfg.MattermostBotHTTPAddr,
		"mattermost_bot_token", cfg.MattermostToken,
	)

	slog.Info("Connecting to Tarantool...")
	tarantoolConfig := tarantool.Opts{
		User:          cfg.TarantoolUser,
		Pass:          cfg.TarantoolPass,
		Timeout:       5 * time.Second,
		Reconnect:     1 * time.Second,
		MaxReconnects: 5,
	}

	var tarantoolStore *db.TarantoolStorage
	var err error

	for attempts := 1; attempts <= 3; attempts++ {
		slog.Info("Connection attempt", "attempt", attempts)

		tarantoolStore, err = db.NewTarantoolStorage(
			cfg.TarantoolAddr,
			tarantoolConfig,
		)

		if err == nil {
			break
		}

		slog.Error("Failed to connect to Tarantool", "error", err, "attempt", attempts)

		if attempts < 3 {
			slog.Info("Retrying in 2 seconds...")
			time.Sleep(2 * time.Second)
		}
	}

	if err != nil {
		slog.Error("All connection attempts to Tarantool failed", "error", err)
		os.Exit(1)
	}

	defer func() {
		if tarantoolStore != nil {
			if err := tarantoolStore.Close(); err != nil {
				slog.Error("Error closing Tarantool connection", "error", err)
			}
		}
	}()

	slog.Info("Connected to Tarantool successfully")

	botService := service.NewService(tarantoolStore, nil)

	slog.Info("Connecting to Mattermost...")

	mmClient, err := mattermost.NewClient(cfg.MattermostConfig, botService)
	if err != nil {
		slog.Error("Failed to connect to Mattermost", "error", err)
		os.Exit(1)
	}

	botService.SetNotifier(mmClient)

	defer mmClient.Close()

	slog.Info("Connected to Mattermost successfully")

	if err := mmClient.RegisterCommands(cfg.MattermostConfig); err != nil {
		slog.Error("Failed to register commands", "error", err)
		os.Exit(1)
	}

	mmClient.StartListening()

	slog.Info("Bot is now running. Press CTRL+C to exit.")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	slog.Info("Shutting down bot...")
}
