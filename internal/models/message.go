package models

import (
	"time"

	"github.com/google/uuid"
)

type Message struct {
	ID            string    `json:"message_id" db:"id"`
	SenderID      string    `json:"sender_id" db:"sender_id"`
	ReceiverID    string    `json:"receiver_id" db:"receiver_id"`
	Content       string    `json:"content" db:"content"`
	Timestamp     time.Time `json:"timestamp" db:"timestamp"`
	Read          bool      `json:"read" db:"read"`
	RedisStreamID string    `json:"-" db:"-"` // Used internally for Redis Stream acknowledgment, not stored in DB
}

func NewMessage(senderID, receiverID, content string) *Message {
	return &Message{
		ID:         uuid.New().String(),
		SenderID:   senderID,
		ReceiverID: receiverID,
		Content:    content,
		Timestamp:  time.Now().UTC(),
		Read:       false,
	}
}

type MessageRequest struct {
	SenderID   string `json:"sender_id" binding:"required"`
	ReceiverID string `json:"receiver_id" binding:"required"`
	Content    string `json:"content" binding:"required"`
}

type ReadStatus struct {
	Status string `json:"status"`
}

type PaginationParams struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

type PaginatedResponse struct {
	Data       []Message       `json:"data"`
	Pagination PaginationMeta `json:"pagination"`
}

type PaginationMeta struct {
	TotalCount int `json:"total_count"`
	HasMore    bool `json:"has_more"`
	Offset     int `json:"offset"`
	Limit      int `json:"limit"`
}
