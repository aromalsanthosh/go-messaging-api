package db

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aromalsanthosh/go-messaging-api/internal/models"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq" 
)

type Database struct {
	db *sqlx.DB
}

// Connect establishes a connection to the PostgreSQL database
func Connect(databaseURL string) (*Database, error) {
	db, err := sqlx.Connect("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Connection pool settings
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Initialize the DB schema
	if err := initSchema(db); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return &Database{db: db}, nil
}

func (d *Database) Close() error {
	return d.db.Close()
}

// initSchema creates the necessary tables if they don't exist
func initSchema(db *sqlx.DB) error {
	// Create messages table
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS messages (
			id UUID PRIMARY KEY,
			sender_id TEXT NOT NULL,
			receiver_id TEXT NOT NULL,
			content TEXT NOT NULL,
			timestamp TIMESTAMP WITH TIME ZONE NOT NULL,
			read BOOLEAN NOT NULL DEFAULT FALSE
		)
	`)

	if err != nil {
		return err
	}

	// Create conversation indexes (for efficient retrieval of conversations)
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_messages_conversation ON messages(sender_id, receiver_id, timestamp DESC)`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_messages_conversation_reverse ON messages(receiver_id, sender_id, timestamp DESC)`)
	if err != nil {
		return err
	}

	// Create timestamp index (for time-based queries and sorting)
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_messages_created_at ON messages(timestamp DESC)`)
	if err != nil {
		return err
	}

	// Create read status index (for filtering unread messages)
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_messages_read_status ON messages(receiver_id, read)`)
	if err != nil {
		return err
	}

	return err
}

// SaveMessage inserts a new message into the database
func (d *Database) SaveMessage(ctx context.Context, message *models.Message) error {
	messageID := uuid.New().String()
	message.ID = messageID

	_, err := d.db.ExecContext(
		ctx,
		`INSERT INTO messages (id, sender_id, receiver_id, content, timestamp, read) 
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		message.ID,
		message.SenderID,
		message.ReceiverID,
		message.Content,
		message.Timestamp,
		message.Read,
	)

	return err
}

// GetConversation retrieves the conversation history between two users
func (d *Database) GetConversation(ctx context.Context, user1, user2 string, params models.PaginationParams) ([]models.Message, error) {
	var messages []models.Message

	queryBuilder := strings.Builder{}
	queryBuilder.WriteString(`SELECT id, sender_id, receiver_id, content, timestamp, read 
		 FROM messages 
		 WHERE ((sender_id = $1 AND receiver_id = $2) OR (sender_id = $2 AND receiver_id = $1))`)

	args := []interface{}{user1, user2}

	queryBuilder.WriteString(` ORDER BY timestamp DESC`)
	
	// Set default limit if not provided
	limit := 20 
	if params.Limit > 0 {
		limit = params.Limit
	}
	
	// Add LIMIT clause
	queryBuilder.WriteString(fmt.Sprintf(` LIMIT $%d`, len(args)+1))
	args = append(args, limit)
	
	// Add OFFSET clause if provided
	if params.Offset > 0 {
		queryBuilder.WriteString(fmt.Sprintf(` OFFSET $%d`, len(args)+1))
		args = append(args, params.Offset)
	}

	query := queryBuilder.String()

	fmt.Printf("Query: %s\n", query)
	fmt.Printf("Args: %+v\n", args)

	err := d.db.SelectContext(ctx, &messages, query, args...)

	return messages, err
}

// GetConversationCount returns the total count of messages between two users
func (d *Database) GetConversationCount(ctx context.Context, user1, user2 string) (int, error) {
	var count int

	err := d.db.GetContext(
		ctx,
		&count,
		`SELECT COUNT(*) FROM messages 
		 WHERE (sender_id = $1 AND receiver_id = $2) OR (sender_id = $2 AND receiver_id = $1)`,
		user1,
		user2,
	)

	return count, err
}

// MarkMessageAsRead marks a message as read
func (d *Database) MarkMessageAsRead(ctx context.Context, messageID string) error {
	fmt.Printf("Marking message as read: %s\n", messageID)
	
	query := "UPDATE messages SET read = true WHERE id = :id"
	
	params := map[string]interface{}{
		"id": messageID,
	}
	
	result, err := d.db.NamedExecContext(ctx, query, params)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("message with ID %s not found", messageID)
	}

	return nil
}
