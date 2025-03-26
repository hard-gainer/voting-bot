package mattermost

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/hard-gainer/voting-bot/internal/api"
	"github.com/hard-gainer/voting-bot/internal/config"
	domain "github.com/hard-gainer/voting-bot/internal/model"
	"github.com/mattermost/mattermost-server/v6/model"
)

// constants for the client
const (
	ReconnectDelay = 5 * time.Second
)

// PollHandler is an polling interface
type PollHandler interface {
	CreatePoll(ctx context.Context, title string, options []string, creatorID string) (*domain.Poll, error)
	GetPoll(ctx context.Context, pollID string) (*domain.Poll, error)
	HandleVote(ctx context.Context, pollID, option, userID string) error
	GetResults(ctx context.Context, pollID string) (map[string]int, error)
	EndPoll(ctx context.Context, pollID, userID string) error
	DeletePoll(ctx context.Context, pollID, userID string) error
	ListPolls(ctx context.Context) ([]*domain.Poll, error)
	FormatPollResults(ctx context.Context, pollID string) (string, error)
}

// Client provides a client for work with Mattermost API
type Client struct {
	client          *model.Client4
	botUser         *model.User
	handlers        map[string]CommandHandler
	webSocketClient *model.WebSocketClient
	pollHandler     PollHandler
	httpHandler     *api.HTTPHandler
}

type MattermostAPI interface {
	GetMe(etag string) (*model.User, *model.Response, error)
	CreateCommand(cmd *model.Command) (*model.Command, *model.Response, error)
	CreatePost(post *model.Post) (*model.Post, *model.Response, error)
}

type WebSocketClient interface {
	Listen()
	EventChannel() chan *model.WebSocketEvent
	Close() error
	Connect() error
}

// CommandHandler defines a function command handler
type CommandHandler func(args []string, userID, channelID string) (string, error)

type CommandRequest struct {
	Command   string   `json:"command"`
	Args      []string `json:"args"`
	UserID    string   `json:"user_id"`
	ChannelID string   `json:"channel_id"`
}

type CommandResponse struct {
	ResponseType string `json:"response_type"`
	Text         string `json:"text"`
}

// NewClient creates a new client Mattermost
func NewClient(cfg config.MattermostConfig, handler PollHandler) (*Client, error) {
	apiClient := model.NewAPIv4Client(cfg.MattermostBotURL)
	apiClient.SetToken(cfg.MattermostToken)

	botUser, _, err := apiClient.GetMe("")
	if err != nil {
		return nil, fmt.Errorf("failed to get bot user: %w", err)
	}

	slog.Info("Connected as bot user", "username", botUser.Username)

	wsURL := strings.Replace(cfg.MattermostBotURL, "http", "ws", 1)
	wsClient, err := model.NewWebSocketClient4(wsURL, cfg.MattermostToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create WebSocket client: %w", err)
	}

	if err := wsClient.Connect(); err != nil {
		return nil, fmt.Errorf("WebSocket connection failed: %w", err)
	}

	client := &Client{
		client:          apiClient,
		botUser:         botUser,
		webSocketClient: wsClient,
		pollHandler:     handler,
		handlers:        make(map[string]CommandHandler),
	}

	client.RegisterCommandHandlers()

	httpHandler := api.NewHTTPHandler(cfg, client)
	httpHandler.Start()
	client.httpHandler = httpHandler

	go client.monitorWebSocket()

	return client, nil
}

// monitorWebSocket monitors the connection and reconnects if necessary
func (c *Client) monitorWebSocket() {
	for event := range c.webSocketClient.EventChannel {
		c.handleEvent(event)
	}

	slog.Warn("WebSocket channel closed, reconnecting...")
	c.reconnectWebSocket()

	go c.monitorWebSocket()
}

func (c *Client) reconnectWebSocket() {
	for {
		time.Sleep(ReconnectDelay)
		if err := c.webSocketClient.Connect(); err == nil {
			slog.Info("WebSocket reconnected successfully")
			return
		}
		slog.Error("WebSocket reconnect failed, retrying...")
	}
}

// RegisterCommandHandlers registers command handlers
func (c *Client) RegisterCommandHandlers() {
	c.RegisterCommandHandler("poll-create", c.handlePollCreate)
	c.RegisterCommandHandler("poll-vote", c.handlePollVote)
	c.RegisterCommandHandler("poll-results", c.handlePollResults)
	c.RegisterCommandHandler("poll-end", c.handlePollEnd)
	c.RegisterCommandHandler("poll-delete", c.handlePollDelete)
	c.RegisterCommandHandler("poll-list", c.handlePollList)
}

// RegisterCommandHandler registers command handler
func (c *Client) RegisterCommandHandler(command string, handler CommandHandler) {
	slog.Info("Registering handler for command", "command", command)
	c.handlers[command] = handler
}

// RegisterCommands registers bot commands in Mattermost
func (c *Client) RegisterCommands(cfg config.MattermostConfig) error {
	commandsEndpoint := fmt.Sprintf("http://%s%s/commands", "voting-bot", cfg.MattermostBotHTTPPort)

	slog.Info("ENDPOINT", "url", commandsEndpoint)

	commands := []*model.Command{
		{
			Trigger:          "poll-create",
			Method:           "P",
			AutoComplete:     true,
			AutoCompleteDesc: "Create a new poll: /poll-create \"Title\" \"Option 1\" \"Option 2\" ...",
			AutoCompleteHint: "Title \"Option 1\" \"Option 2\" ...",
			URL:              commandsEndpoint,
		},
		{
			Trigger:          "poll-vote",
			Method:           "P",
			AutoComplete:     true,
			AutoCompleteDesc: "Vote in a poll: /poll-vote poll-id option",
			AutoCompleteHint: "poll-id option",
			URL:              commandsEndpoint,
		},
		{
			Trigger:          "poll-results",
			Method:           "P",
			AutoComplete:     true,
			AutoCompleteDesc: "Show poll results: /poll-results poll-id",
			AutoCompleteHint: "poll-id",
			URL:              commandsEndpoint,
		},
		{
			Trigger:          "poll-end",
			Method:           "P",
			AutoComplete:     true,
			AutoCompleteDesc: "End a poll: /poll-end poll-id",
			AutoCompleteHint: "poll-id",
			URL:              commandsEndpoint,
		},
		{
			Trigger:          "poll-delete",
			Method:           "P",
			AutoComplete:     true,
			AutoCompleteDesc: "Delete a poll: /poll-delete poll-id",
			AutoCompleteHint: "poll-id",
			URL:              commandsEndpoint,
		},
		{
			Trigger:          "poll-list",
			Method:           "P",
			AutoComplete:     true,
			AutoCompleteDesc: "List all active polls",
			AutoCompleteHint: "",
			URL:              commandsEndpoint,
		},
	}

	// c.CheckBotPermissions()
	teams, _, err := c.client.GetTeamsForUser(c.botUser.Id, "")
	if err != nil {
		return fmt.Errorf("failed to get teams: %w", err)
	}

	for _, team := range teams {
		existingCommands, _, err := c.client.ListCommands(team.Id, true)
		if err != nil {
			slog.Error("Failed to get existing commands", "team", team.Name, "error", err)
		}

		if len(existingCommands) > 0 {
			continue
		}

		for _, cmd := range commands {
			cmd.TeamId = team.Id
			cmd.CreatorId = c.botUser.Id

			_, _, err := c.client.CreateCommand(cmd)
			if err != nil {
				return fmt.Errorf("failed to register command %s: %w", cmd.Trigger, err)
			}

			slog.Info("Registered command", "trigger", cmd.Trigger)
		}
	}
	return nil
}

// StartListening starts listening for WebSocket events
func (c *Client) StartListening() {
	c.webSocketClient.Listen()

	go func() {
		eventChannel := c.webSocketClient.EventChannel
		if eventChannel == nil {
			slog.Error("Failed to get event channel")
			return
		}

		for event := range eventChannel {
			c.handleEvent(event)
		}
	}()

	slog.Info("Started listening for WebSocket events")
}

// handleEvent handles WebSocket event
func (c *Client) handleEvent(event *model.WebSocketEvent) {
	switch event.EventType() {
	case "slash_command":
		c.handleSlashCommand(event)
	}
}

// handleSlashCommand handles commands
func (c *Client) handleSlashCommand(event *model.WebSocketEvent) {
	data := event.GetData()
	command, ok := data["command"].(string)
	if !ok {
		return
	}

	parts := strings.Fields(command)
	if len(parts) == 0 {
		return
	}

	commandName := strings.TrimPrefix(parts[0], "/")
	handler, exists := c.handlers[commandName]
	if !exists {
		return
	}

	userID, ok := data["user_id"].(string)
	if !ok {
		userID = event.GetBroadcast().UserId
	}

	channelID, ok := data["channel_id"].(string)
	if !ok {
		channelID = event.GetBroadcast().ChannelId
	}

	response, err := handler(parts[1:], userID, channelID)
	if err != nil {
		c.PostMessage(channelID, fmt.Sprintf("Error: %v", err))
		return
	}

	if response != "" {
		c.PostMessage(channelID, response)
	}
}

// handlePollCreate handles the creation of the poll
func (c *Client) handlePollCreate(args []string, userID, channelID string) (string, error) {
	if len(args) < 3 {
		return "Usage: `/poll-create \"Title\" \"Option 1\" \"Option 2\" ...`\nTitle" +
			"and at least 2 options enclosed with \"\" are required.", nil
	}

	title := args[0]
	options := args[1:]

	if len(options) < 2 {
		return "Error: Please provide a title and at least 2 options.", nil
	}

	slog.Info("Creating poll", "title", title, "options", options)

	ctx := context.Background()
	poll, err := c.pollHandler.CreatePoll(ctx, title, options, userID)
	if err != nil {
		return "", fmt.Errorf("failed to create poll: %w", err)
	}

	response := fmt.Sprintf("### Poll Created: %s\n\n**ID:** %s\n\n**Options:**\n", poll.Title, poll.ID)
	for i, option := range poll.Options {
		response += fmt.Sprintf("%d. %s\n", i+1, option)
	}

	response += fmt.Sprintf("\nTo vote: `/poll-vote %s \"Option\"`\nTo see results: `/poll-results %s`", poll.ID, poll.ID)

	return response, nil
}

// handlePollVote handles poll voting
func (c *Client) handlePollVote(args []string, userID, channelID string) (string, error) {
	if len(args) < 2 {
		return "Usage: `/poll-vote [poll-id] [option]`", nil
	}

	pollID := args[0]
	option := args[1]

	ctx := context.Background()
	if err := c.pollHandler.HandleVote(ctx, pollID, option, userID); err != nil {
		return "", fmt.Errorf("failed to vote: %w", err)
	}

	return fmt.Sprintf("Your vote for **%s** in poll **%s** has been recorded.", option, pollID), nil
}

// handlePollResults handles results display of the poll
func (c *Client) handlePollResults(args []string, userID, channelID string) (string, error) {
	if len(args) < 1 {
		return "Usage: `/poll-results [poll-id]`", nil
	}

	pollID := args[0]
	ctx := context.Background()

	results, err := c.pollHandler.FormatPollResults(ctx, pollID)
	if err != nil {
		return "", fmt.Errorf("failed to get poll results: %w", err)
	}

	return results, nil
}

// handlePollEnd ends a poll
func (c *Client) handlePollEnd(args []string, userID, channelID string) (string, error) {
	if len(args) < 1 {
		return "Usage: `/poll-end [poll-id]`", nil
	}

	pollID := args[0]
	ctx := context.Background()

	if err := c.pollHandler.EndPoll(ctx, pollID, userID); err != nil {
		return "", fmt.Errorf("failed to end poll: %w", err)
	}

	results, err := c.pollHandler.FormatPollResults(ctx, pollID)
	if err != nil {
		return "Poll has been ended, but results could not be displayed.", nil
	}

	return fmt.Sprintf("Poll has been ended.\n\n%s", results), nil
}

// handlePollDelete handles poll deletion
func (c *Client) handlePollDelete(args []string, userID, channelID string) (string, error) {
	if len(args) < 1 {
		return "Usage: `/poll-delete [poll-id]`", nil
	}

	pollID := args[0]
	ctx := context.Background()

	poll, err := c.pollHandler.GetPoll(ctx, pollID)
	if err != nil {
		return "", fmt.Errorf("failed to get poll: %w", err)
	}

	if err := c.pollHandler.DeletePoll(ctx, pollID, userID); err != nil {
		return "", fmt.Errorf("failed to delete poll: %w", err)
	}

	return fmt.Sprintf("Poll **%s: %s** has been deleted.", pollID, poll.Title), nil
}

// handlePollList prints list of all polls
func (c *Client) handlePollList(args []string, userID, channelID string) (string, error) {
	ctx := context.Background()
	polls, err := c.pollHandler.ListPolls(ctx)

	if err != nil {
		return "", fmt.Errorf("failed to list polls: %w", err)
	}

	if len(polls) == 0 {
		return "No polls found.", nil
	}

	response := "### Available Polls\n\n"

	for i, poll := range polls {
		status := "Active"
		if !poll.IsActive {
			status = "Closed"
		}

		voteCount := len(poll.Votes)

		response += fmt.Sprintf("%d. **%s** (ID: `%s`)\n", i+1, poll.Title, poll.ID)
		response += fmt.Sprintf("   Status: %s | Votes: %d\n\n", status, voteCount)
	}

	return response, nil
}

// PostMessage posts message to the channel
func (c *Client) PostMessage(channelID, message string) error {
	post := &model.Post{
		UserId:    c.botUser.Id,
		ChannelId: channelID,
		Message:   message,
	}

	_, _, err := c.client.CreatePost(post)
	if err != nil {
		slog.Error("Failed to post message", "error", err)
		return err
	}

	return nil
}

// Close closes WebSocket connection
func (c *Client) Close() {
	if c.webSocketClient != nil {
		c.webSocketClient.Close()
		slog.Info("WebSocket client closed")
	}

	if c.httpHandler != nil {
		if err := c.httpHandler.Stop(); err != nil {
			slog.Error("Failed to stop HTTP handler", "error", err)
		}
	}

	slog.Info("Mattermost client connections closed")
}

// HandleCommand implements interface PollCommandHandler
func (c *Client) HandleCommand(command string, args []string, userID, channelID string) (string, error) {
	commandName := strings.TrimPrefix(command, "/")

	handler, exists := c.handlers[commandName]
	if !exists {
		return "", fmt.Errorf("unknown command: %s", commandName)
	}

	return handler(args, userID, channelID)
}
