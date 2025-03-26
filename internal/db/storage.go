package db

import (
	"context"

	"github.com/hard-gainer/voting-bot/internal/model"
)

type Storage interface {
	CreatePoll(ctx context.Context, poll *model.Poll) error
	GetPoll(ctx context.Context, id string) (*model.Poll, error)
	UpdatePoll(ctx context.Context, poll *model.Poll) error
	DeletePoll(ctx context.Context, id string) error
	ListPolls(ctx context.Context) ([]*model.Poll, error)
    Close() error
}
