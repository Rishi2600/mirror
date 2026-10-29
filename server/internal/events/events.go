// Package events is the one writer for tables in the events schema. Keeping
// every write here means the events tables can move to their own database
// later with a change in one place.
package events

import (
	"context"
	"time"

	"gorm.io/gorm"
)

// AICall is one LLM request, logged for cost and debugging.
type AICall struct {
	ID           int64 // set by the database
	Purpose      string
	Model        string
	InputTokens  int
	OutputTokens int
	LatencyMS    int `gorm:"column:latency_ms"`
	OK           bool
	Error        string
	RawOutput    string
	CreatedAt    time.Time
}

func (AICall) TableName() string { return "events.ai_calls" }

// LogAICall records an AI call. Callers log a failure here and carry on.
func LogAICall(ctx context.Context, db *gorm.DB, call AICall) error {
	return db.WithContext(ctx).Create(&call).Error
}

// Notification is one in-app alert sent to a user.
type Notification struct {
	ID        int64 // set by the database
	UserID    string
	Channel   string
	Kind      string
	MatchID   string
	Delivered bool
	CreatedAt time.Time
}

func (Notification) TableName() string { return "events.notifications" }

// LogNotification records a notification. Callers log a failure here and
// carry on.
func LogNotification(ctx context.Context, db *gorm.DB, n Notification) error {
	return db.WithContext(ctx).Create(&n).Error
}
