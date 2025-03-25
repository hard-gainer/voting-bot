package main

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/hard-gainer/voting-bot/internal/config"
	"github.com/hard-gainer/voting-bot/internal/db"
	"github.com/hard-gainer/voting-bot/internal/logger"
	"github.com/tarantool/go-tarantool"
)

func main() {
	logger.InitLogger()
	slog.Info("Starting Mattermost voting bot...")

	cfg := config.NewConfig()

	slog.Info("Connecting to Tarantool...")
	tarantoolConfig := tarantool.Opts{
		User: cfg.TarantoolUser,
		Pass: cfg.TarantoolPass,
	}
	tarantoolStore, err := db.NewTarantoolStorage(
		cfg.TarantoolAddr,
		tarantoolConfig,
	)

	if err != nil {
		slog.Error("Failed to connect to Tarantool", "error", err)
	}
	defer tarantoolStore.Close()

	slog.Info("Connected to Tarantool successfully")

	// slog.Info("Connecting to Mattermost...")
	// mmClient, err := mattermost.NewClient(mattermost.Config{
	// 	ServerURL: cfg.MattermostURL,
	// 	BotToken:  cfg.MattermostToken,
	// })
	// if err != nil {
	// 	slog.Error("Failed to connect to Mattermost: %v", err)
	// }
	// defer mmClient.Close()
	// slog.Info("Connected to Mattermost successfully")

	// botService := service.NewService(tarantoolStore, mmClient)

	// if err := mmClient.RegisterCommands(); err != nil {
	// 	slog.Error("Failed to register commands: %v", err)
	// }

	// mmClient.StartListening()
	slog.Info("Bot is now running. Press CTRL+C to exit.")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	slog.Info("Shutting down bot...")
}
