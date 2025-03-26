package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/hard-gainer/voting-bot/internal/config"
)

// PollCommandHandler represents an interface for handling command polls
type PollCommandHandler interface {
	HandleCommand(command string, args []string, userID, channelID string) (string, error)
}

// CommandRequest  represents a request to execute a command
type CommandRequest struct {
	Command     string   `json:"command" form:"command"`
	Text        string   `json:"text" form:"text"`
	Args        []string `json:"args" form:"-"`
	UserID      string   `json:"user_id" form:"user_id"`
	ChannelID   string   `json:"channel_id" form:"channel_id"`
	TeamID      string   `json:"team_id" form:"team_id"`
	TeamDomain  string   `json:"team_domain" form:"team_domain"`
	ResponseURL string   `json:"response_url" form:"response_url"`
}

// CommandResponse represents an answer to execute a command
type CommandResponse struct {
	ResponseType string `json:"response_type"`
	Text         string `json:"text"`
}

// HTTPHandler serves HTTP request for handling commands
type HTTPHandler struct {
	server         *http.Server
	commandHandler PollCommandHandler
}

// NewHTTPHandler creates a new HTTP-handler for request
func NewHTTPHandler(cfg config.MattermostConfig, handler PollCommandHandler) *HTTPHandler {
	h := &HTTPHandler{
		commandHandler: handler,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /commands", h.handleCommand)

	h.server = &http.Server{
		Addr:         strings.TrimPrefix(cfg.MattermostBotHTTPPort, "http://"),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return h
}

// Start starts an HTTP-server
func (h *HTTPHandler) Start() {
	go func() {
		slog.Info("Starting HTTP server", "address", h.server.Addr)
		if err := h.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP server failed", "error", err)
		}
	}()

	slog.Info("HTTP server started")
}

// Stop stops an HTTP-server
func (h *HTTPHandler) Stop() error {
	slog.Info("Shutting down HTTP server")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return h.server.Shutdown(ctx)
}

// handleCommand handles command requests
func (h *HTTPHandler) handleCommand(w http.ResponseWriter, r *http.Request) {
	slog.Info("Received command request",
		"method", r.Method,
		"content-type", r.Header.Get("Content-Type"),
		"url", r.URL.String())

	var commandName string
	var args []string
	var userID, channelID string

	contentType := r.Header.Get("Content-Type")

	if strings.Contains(contentType, "application/json") {
		var req CommandRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			slog.Error("Failed to parse JSON request", "error", err)
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}
		commandName = req.Command
		args = req.Args
		userID = req.UserID
		channelID = req.ChannelID
	} else {
		if err := r.ParseForm(); err != nil {
			slog.Error("Failed to parse form data", "error", err)
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		for key, values := range r.Form {
			slog.Debug("Form parameter", "key", key, "values", values)
		}

		commandName = r.Form.Get("command")
		text := r.Form.Get("text")
		userID = r.Form.Get("user_id")
		channelID = r.Form.Get("channel_id")

		if text != "" {
			args = parseCommandArgs(text)
			slog.Debug("Parsed arguments", "count", len(args), "args", args)
		}
	}

	commandName = strings.TrimPrefix(commandName, "/")

	slog.Info("Processing command",
		"command", commandName,
		"args", args,
		"user_id", userID,
		"channel_id", channelID)

	response, err := h.commandHandler.HandleCommand(commandName, args, userID, channelID)
	if err != nil {
		slog.Error("Failed to handle command", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(CommandResponse{
		ResponseType: "in_channel",
		Text:         response,
	})
}

// parseCommandArgs parses command arguments enclosed in double quotes
func parseCommandArgs(text string) []string {
	if text == "" {
		return nil
	}

	var args []string
	var currentArg strings.Builder
	inQuotes := false

	for i := 0; i < len(text); i++ {
		char := text[i]

		switch char {
		case '"':
			inQuotes = !inQuotes

			if !inQuotes && currentArg.Len() > 0 {
				args = append(args, currentArg.String())
				currentArg.Reset()
			}
		case ' ', '\t', '\n', '\r':
			if inQuotes {
				currentArg.WriteByte(char)
			} else if currentArg.Len() > 0 {
				args = append(args, currentArg.String())
				currentArg.Reset()
			}
		default:
			currentArg.WriteByte(char)
		}
	}

	if currentArg.Len() > 0 {
		args = append(args, currentArg.String())
	}

	return args
}
