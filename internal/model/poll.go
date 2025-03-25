package model

import "time"

type Poll struct {
	ID        string            `json:"id"`
	Title     string            `json:"title"`
	Options   []string          `json:"options"`
	CreatedBy string            `json:"created_by"`
	CreatedAt time.Time         `json:"created_at"`
	IsActive  bool              `json:"is_active"`
	Votes     map[string]string `json:"votes"` // map[user_id] = option_index
}
