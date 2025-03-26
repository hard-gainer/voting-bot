package db

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/hard-gainer/voting-bot/internal/model"
	"github.com/tarantool/go-tarantool"
)

var (
	ErrNotFound = errors.New("record not found")
)

// TarantoolStorage implements the db.Storage interface using Tarantool
type TarantoolStorage struct {
	conn *tarantool.Connection
}

// NewTarantoolStorage creates a new Tarantool storage instance
func NewTarantoolStorage(addr string, opts tarantool.Opts) (*TarantoolStorage, error) {
	conn, err := tarantool.Connect(addr, opts)
	if err != nil {
		return nil, err
	}

	return &TarantoolStorage{
		conn: conn,
	}, nil
}

// CreatePoll stores a new poll in Tarantool
func (s *TarantoolStorage) CreatePoll(ctx context.Context, poll *model.Poll) error {
	slog.Info("Storing poll in Tarantool", "poll_id", poll.ID)
	poll.CreatedAt = uint64(time.Now().Unix())

	_, err := s.conn.Insert(
		"polls",
		[]interface{}{
			poll.ID, poll.Title,
			poll.Options, poll.CreatedBy,
			poll.CreatedAt, poll.IsActive,
			poll.Votes,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to insert poll: %w", err)
	}

	return nil
}

// GetPoll retrieves a poll from Tarantool
func (s *TarantoolStorage) GetPoll(ctx context.Context, id string) (*model.Poll, error) {
	slog.Info("Retrieving poll from Tarantool", "poll_id", id)

	resp, err := s.conn.Select("polls", "primary", 0, 1, tarantool.IterEq, []interface{}{id})
	if err != nil {
		return nil, fmt.Errorf("tarantool select error: %w", err)
	}

	if len(resp.Data) == 0 {
		return nil, ErrNotFound
	}

	data, ok := resp.Data[0].([]interface{})
	if !ok || len(data) < 7 {
		return nil, fmt.Errorf("invalid Tarantool response")
	}

	return &model.Poll{
		ID:        data[0].(string),
		Title:     data[1].(string),
		Options:   convertToStringSlice(data[2]),
		CreatedBy: data[3].(string),
		CreatedAt: data[4].(uint64),
		IsActive:  data[5].(bool),
		Votes:     convertToMapStringString(data[6]),
	}, nil
}

// UpdatePoll updates an existing poll in Tarantool
func (s *TarantoolStorage) UpdatePoll(ctx context.Context, poll *model.Poll) error {
	slog.Info("Updating poll in Tarantool", "poll_id", poll.ID)

	_, err := s.conn.Replace(
		"polls",
		[]interface{}{
			poll.ID, poll.Title,
			poll.Options, poll.CreatedBy,
			poll.CreatedAt, poll.IsActive,
			poll.Votes,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to update poll: %w", err)
	}

	return nil
}

// DeletePoll removes a poll from Tarantool
func (s *TarantoolStorage) DeletePoll(ctx context.Context, id string) error {
	slog.Info("Deleting poll from Tarantool", "poll_id", id)

	_, err := s.conn.Delete("polls", "primary", []interface{}{id})
	if err != nil {
		return fmt.Errorf("failed to delete poll: %w", err)
	}

	return nil
}

// ListPolls lists all polls in Tarantool
func (s *TarantoolStorage) ListPolls(ctx context.Context) ([]*model.Poll, error) {
	slog.Info("Listing all polls from Tarantool")

	resp, err := s.conn.Select("polls", "primary", 0, 1000, tarantool.IterAll, []interface{}{})
	if err != nil {
		return nil, fmt.Errorf("tarantool select error: %w", err)
	}

	polls := make([]*model.Poll, 0, len(resp.Data))

	for _, tupleData := range resp.Data {
		data, ok := tupleData.([]interface{})
		if !ok || len(data) < 7 {
			slog.Info("Warning: invalid tuple format")
			continue
		}

		poll := &model.Poll{
			ID:        data[0].(string),
			Title:     data[1].(string),
			Options:   convertToStringSlice(data[2]),
			CreatedBy: data[3].(string),
			CreatedAt: data[4].(uint64),
			IsActive:  data[5].(bool),
			Votes:     convertToMapStringString(data[6]),
		}

		polls = append(polls, poll)
	}

	return polls, nil
}

// Close closes the Tarantool connection
func (s *TarantoolStorage) Close() error {
	return s.conn.Close()
}

// convertToStringSlice is a helper function for converting to string slice
func convertToStringSlice(value interface{}) []string {
	slice, ok := value.([]interface{})
	if !ok {
		return nil
	}
	result := make([]string, len(slice))
	for i, v := range slice {
		result[i] = v.(string)
	}
	return result
}

// convertToMapString is a helper function for converting to map[string]string
func convertToMapStringString(value interface{}) map[string]string {
	m, ok := value.(map[interface{}]interface{})
	if !ok {
		return nil
	}
	result := make(map[string]string)
	for k, v := range m {
		key, ok1 := k.(string)
		val, ok2 := v.(string)
		if !ok1 || !ok2 {
			continue
		}
		result[key] = val
	}
	return result
}
