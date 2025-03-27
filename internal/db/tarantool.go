package db

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/hard-gainer/voting-bot/internal/model"
	"github.com/tarantool/go-tarantool"
	pool "github.com/tarantool/go-tarantool/connection_pool"
)

var (
	ErrNotFound = errors.New("record not found")
)

// Storage defines the methods for working with the poll storage
type Storage interface {
	// CreatePoll saves a new poll in Tarantool
	CreatePoll(ctx context.Context, poll *model.Poll) error
	// GetPoll retrieves a poll from Tarantool
	GetPoll(ctx context.Context, id string) (*model.Poll, error)
	// UpdatePoll updates an existing poll in Tarantool
	UpdatePoll(ctx context.Context, poll *model.Poll) error
	// DeletePoll removes a poll from Tarantool
	DeletePoll(ctx context.Context, id string) error
	// ListPolls lists all polls in Tarantool
	ListPolls(ctx context.Context) ([]*model.Poll, error)
	// Close closes the Tarantool connection
	Close() error
}

// TarantoolStorage implements the Storage interface using Tarantool
type TarantoolStorage struct {
	connPool *pool.ConnectionPool
}

// NewTarantoolStorage creates a new Tarantool storage instance with connection pool
func NewTarantoolStorage(addr string, opts tarantool.Opts) (*TarantoolStorage, error) {
    slog.Info("Connecting to Tarantool", "addr", addr)

    poolOpts := pool.OptsPool{
        CheckTimeout:      1 * time.Second,
    }

    connPool, err := pool.ConnectWithOpts([]string{addr}, opts, poolOpts)
    if err != nil {
        return nil, fmt.Errorf("failed to create connection pool: %w", err)
    }

    _, err = connPool.Call("box.space.polls:len", []interface{}{}, pool.ANY)
    if err != nil {
        connPool.Close()
        return nil, fmt.Errorf("failed to verify polls space: %w", err)
    }

    slog.Info("Successfully connected to Tarantool")
    return &TarantoolStorage{
        connPool: connPool,
    }, nil
}

// CreatePoll saves a new poll in Tarantool
func (s *TarantoolStorage) CreatePoll(ctx context.Context, poll *model.Poll) error {
    slog.Info("Storing poll in Tarantool", "poll_id", poll.ID)

    if poll.CreatedAt == 0 {
        poll.CreatedAt = uint64(time.Now().Unix())
    }

    _, err := s.connPool.Insert(
        "polls",
        []interface{}{
            poll.ID, 
            poll.Title,
            poll.Options, 
            poll.CreatedBy,
            poll.CreatedAt, 
            poll.IsActive,
            poll.Votes,
        },
        pool.RW,
    )
    if err != nil {
        return fmt.Errorf("failed to insert poll: %w", err)
    }

    return nil
}

// GetPoll retrieves a poll from Tarantool
func (s *TarantoolStorage) GetPoll(ctx context.Context, id string) (*model.Poll, error) {
    slog.Info("Retrieving poll from Tarantool", "poll_id", id)

    // Используем любое доступное соединение для чтения
    resp, err := s.connPool.Select("polls", "primary", 0, 1, tarantool.IterEq, []interface{}{id}, pool.ANY)
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

    _, err := s.connPool.Replace(
        "polls",
        []interface{}{
            poll.ID, 
            poll.Title,
            poll.Options, 
            poll.CreatedBy,
            poll.CreatedAt, 
            poll.IsActive,
            poll.Votes,
        },
        pool.RW,
    )
    if err != nil {
        return fmt.Errorf("failed to update poll: %w", err)
    }

    return nil
}

// DeletePoll removes a poll from Tarantool
func (s *TarantoolStorage) DeletePoll(ctx context.Context, id string) error {
    slog.Info("Deleting poll from Tarantool", "poll_id", id)

    _, err := s.connPool.Delete("polls", "primary", []interface{}{id}, pool.RW)
    if err != nil {
        return fmt.Errorf("failed to delete poll: %w", err)
    }

    return nil
}

// ListPolls lists all polls in Tarantool
func (s *TarantoolStorage) ListPolls(ctx context.Context) ([]*model.Poll, error) {
    slog.Info("Listing all polls from Tarantool")

    // Используем любое соединение для чтения
    resp, err := s.connPool.Select("polls", "primary", 0, 1000, tarantool.IterAll, []interface{}{}, pool.ANY)
    if err != nil {
        return nil, fmt.Errorf("tarantool select error: %w", err)
    }

    return s.convertResponseToPolls(resp)
}

// convertResponseToPolls converts a Tarantool response to a slice of polls
func (s *TarantoolStorage) convertResponseToPolls(resp *tarantool.Response) ([]*model.Poll, error) {
    polls := make([]*model.Poll, 0, len(resp.Data))

    for _, tupleData := range resp.Data {
        data, ok := tupleData.([]interface{})
        if !ok || len(data) < 7 {
            slog.Warn("Invalid tuple format in Tarantool response", "data", tupleData)
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

// Close closes the Tarantool connection pool
func (s *TarantoolStorage) Close() error {
    slog.Info("Closing Tarantool connection pool")
    errs := s.connPool.Close()
    if len(errs) > 0 {
        return fmt.Errorf("errors closing Tarantool pool: %v", errs)
    }
    return nil
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
