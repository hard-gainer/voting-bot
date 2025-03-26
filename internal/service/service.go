package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/hard-gainer/voting-bot/internal/db"
	"github.com/hard-gainer/voting-bot/internal/model"
	"github.com/hard-gainer/voting-bot/internal/notification"
)

// service errors
var (
	ErrPollNotFound  = errors.New("poll not found")
	ErrPollInactive  = errors.New("poll is not active")
	ErrInvalidOption = errors.New("invalid option")
	ErrNotAuthorized = errors.New("not authorized to perform this action")
	ErrAlreadyVoted  = errors.New("already voted in this poll")
)

// Service represents service layer
type Service struct {
	storage  db.Storage
	notifier notification.MessageSender
}

// NewService creates an instance of service
func NewService(storage db.Storage, notifier notification.MessageSender) *Service {
	return &Service{
		storage:  storage,
		notifier: notifier,
	}
}

// SetNotifier позволяет установить или обновить notifier после создания сервиса
func (s *Service) SetNotifier(notifier notification.MessageSender) {
	s.notifier = notifier
}

// NotifyChannel отправляет сообщение в канал, если notifier настроен
func (s *Service) NotifyChannel(channelID, message string) error {
	if s.notifier == nil {
		slog.Warn("Notifier not configured, message not sent", "channel_id", channelID)
		return nil
	}

	return s.notifier.PostMessage(channelID, message)
}

// CreatePoll creates a new poll
func (s *Service) CreatePoll(ctx context.Context, title string, options []string, creatorID string) (*model.Poll, error) {
	slog.Info("Creating poll", "title", title, "options_count", len(options), "creator", creatorID)

	if title == "" {
		return nil, errors.New("empty poll title")
	}

	if len(options) < 2 {
		return nil, errors.New("poll must have at least two options")
	}

	poll := &model.Poll{
		ID:        uuid.New().String(),
		Title:     title,
		Options:   options,
		CreatedBy: creatorID,
		CreatedAt: uint64(time.Now().Unix()),
		IsActive:  true,
		Votes:     make(map[string]string),
	}

	if err := s.storage.CreatePoll(ctx, poll); err != nil {
		slog.Error("Failed to create poll", "error", err)
		return nil, fmt.Errorf("failed to create poll: %w", err)
	}

	slog.Info("Poll created successfully", "poll_id", poll.ID)
	return poll, nil
}

// GetPoll returns the poll by ID
func (s *Service) GetPoll(ctx context.Context, pollID string) (*model.Poll, error) {
	slog.Info("Getting poll", "poll_id", pollID)

	poll, err := s.storage.GetPoll(ctx, pollID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			slog.Info("Poll not found", "poll_id", pollID)
			return nil, ErrPollNotFound
		}
		slog.Error("Failed to get poll", "poll_id", pollID, "error", err)
		return nil, fmt.Errorf("failed to get poll: %w", err)
	}

	return poll, nil
}

// HandleVote handles user's vote
func (s *Service) HandleVote(ctx context.Context, pollID, option, userID string) error {
	slog.Info("Handling vote", "poll_id", pollID, "option", option, "user_id", userID)

	poll, err := s.storage.GetPoll(ctx, pollID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return ErrPollNotFound
		}
		return fmt.Errorf("failed to get poll: %w", err)
	}

	if !poll.IsActive {
		slog.Info("Attempted to vote in inactive poll", "poll_id", pollID, "user_id", userID)
		return ErrPollInactive
	}

	optionValid := false
	for _, opt := range poll.Options {
		if opt == option {
			optionValid = true
			break
		}
	}

	if !optionValid {
		slog.Info("Invalid option selected", "poll_id", pollID, "option", option, "user_id", userID)
		return ErrInvalidOption
	}

	if existingOption, voted := poll.Votes[userID]; voted {
		slog.Info("User updating vote", "poll_id", pollID, "user_id", userID,
			"old_option", existingOption, "new_option", option)
	}

	if poll.Votes == nil {
		poll.Votes = make(map[string]string)
	}

	poll.Votes[userID] = option

	if err := s.storage.UpdatePoll(ctx, poll); err != nil {
		slog.Error("Failed to update poll with vote", "poll_id", pollID, "user_id", userID, "error", err)
		return fmt.Errorf("failed to update poll: %w", err)
	}

	slog.Info("Vote processed successfully", "poll_id", pollID, "user_id", userID, "option", option)
	return nil
}

// GetResults returns poll results
func (s *Service) GetResults(ctx context.Context, pollID string) (map[string]int, error) {
	slog.Info("Getting results", "poll_id", pollID)

	poll, err := s.storage.GetPoll(ctx, pollID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return nil, ErrPollNotFound
		}
		return nil, fmt.Errorf("failed to get poll: %w", err)
	}

	results := make(map[string]int)

	for _, option := range poll.Options {
		results[option] = 0
	}

	for _, vote := range poll.Votes {
		results[vote]++
	}

	slog.Info("Results calculated", "poll_id", pollID, "results", results)
	return results, nil
}

// EndPoll ends a poll
func (s *Service) EndPoll(ctx context.Context, pollID, userID string) error {
	slog.Info("Ending poll", "poll_id", pollID, "user_id", userID)

	poll, err := s.storage.GetPoll(ctx, pollID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return ErrPollNotFound
		}
		return fmt.Errorf("failed to get poll: %w", err)
	}

	if poll.CreatedBy != userID {
		slog.Info("Unauthorized attempt to end poll", "poll_id", pollID,
			"creator", poll.CreatedBy, "requester", userID)
		return ErrNotAuthorized
	}

	if !poll.IsActive {
		slog.Info("Poll already inactive", "poll_id", pollID)
		return nil
	}

	poll.IsActive = false

	if err := s.storage.UpdatePoll(ctx, poll); err != nil {
		slog.Error("Failed to update poll status", "poll_id", pollID, "error", err)
		return fmt.Errorf("failed to update poll: %w", err)
	}

	slog.Info("Poll ended successfully", "poll_id", pollID)
	return nil
}

// DeletePoll deletes a poll
func (s *Service) DeletePoll(ctx context.Context, pollID, userID string) error {
	slog.Info("Deleting poll", "poll_id", pollID, "user_id", userID)

	poll, err := s.storage.GetPoll(ctx, pollID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return ErrPollNotFound
		}
		return fmt.Errorf("failed to get poll: %w", err)
	}

	if poll.CreatedBy != userID {
		slog.Info("Unauthorized attempt to delete poll", "poll_id", pollID,
			"creator", poll.CreatedBy, "requester", userID)
		return ErrNotAuthorized
	}

	if err := s.storage.DeletePoll(ctx, pollID); err != nil {
		slog.Error("Failed to delete poll", "poll_id", pollID, "error", err)
		return fmt.Errorf("failed to delete poll: %w", err)
	}

	slog.Info("Poll deleted successfully", "poll_id", pollID)
	return nil
}

// ListPolls returns a list of all polls
func (s *Service) ListPolls(ctx context.Context) ([]*model.Poll, error) {
	slog.Info("Listing all polls")

	polls, err := s.storage.ListPolls(ctx)
	if err != nil {
		slog.Error("Failed to list polls", "error", err)
		return nil, fmt.Errorf("failed to list polls: %w", err)
	}

	slog.Info("Polls retrieved successfully", "count", len(polls))
	return polls, nil
}

// GetActivePollsByUser returns active polls created by user
func (s *Service) GetActivePollsByUser(ctx context.Context, userID string) ([]*model.Poll, error) {
	slog.Info("Getting active polls for user", "user_id", userID)

	allPolls, err := s.storage.ListPolls(ctx)
	if err != nil {
		slog.Error("Failed to list polls", "error", err)
		return nil, fmt.Errorf("failed to list polls: %w", err)
	}

	userPolls := make([]*model.Poll, 0)
	for _, poll := range allPolls {
		if poll.CreatedBy == userID && poll.IsActive {
			userPolls = append(userPolls, poll)
		}
	}

	slog.Info("Active user polls retrieved", "user_id", userID, "count", len(userPolls))
	return userPolls, nil
}

// GetPollVoteCounts returns amount of votes in the poll
func (s *Service) GetPollVoteCounts(ctx context.Context, pollID string) (int, error) {
	slog.Info("Getting vote count for poll", "poll_id", pollID)

	poll, err := s.storage.GetPoll(ctx, pollID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return 0, ErrPollNotFound
		}
		return 0, fmt.Errorf("failed to get poll: %w", err)
	}

	voteCount := len(poll.Votes)
	slog.Info("Poll vote count retrieved", "poll_id", pollID, "count", voteCount)
	return voteCount, nil
}

// FormatPollResults formats the poll results
func (s *Service) FormatPollResults(ctx context.Context, pollID string) (string, error) {
	slog.Info("Formatting poll results", "poll_id", pollID)

	poll, err := s.storage.GetPoll(ctx, pollID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return "", ErrPollNotFound
		}
		return "", fmt.Errorf("failed to get poll: %w", err)
	}

	results, err := s.GetResults(ctx, pollID)
	if err != nil {
		return "", err
	}

	formattedResults := fmt.Sprintf("### Poll: %s\n\n", poll.Title)

	if !poll.IsActive {
		formattedResults += "**Status: Closed**\n\n"
	}

	totalVotes := len(poll.Votes)
	formattedResults += fmt.Sprintf("**Total votes: %d**\n\n", totalVotes)

	formattedResults += "#### Results:\n"
	for _, option := range poll.Options {
		votes := results[option]
		var percentage float64 = 0
		if totalVotes > 0 {
			percentage = float64(votes) / float64(totalVotes) * 100
		}
		formattedResults += fmt.Sprintf("- **%s**: %d votes (%.1f%%)\n", option, votes, percentage)
	}

	slog.Info("Results formatted successfully", "poll_id", pollID)
	return formattedResults, nil
}
