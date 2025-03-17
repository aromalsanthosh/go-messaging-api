package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aromalsanthosh/go-messaging-api/internal/models"
	"github.com/go-redis/redis/v8"
)

const (
	MessageStreamKey = "message_stream" // Redis Stream key for messages
	
	ConsumerGroup = "message_processors" // Consumer group name
	
	ConsumerName = "processor_1" // Consumer name
)

// RedisQueue implements a message queue using Redis Streams
type RedisQueue struct {
	client *redis.Client
}

// NewRedisQueue creates a new Redis queue using Redis Streams
func NewRedisQueue(redisURL string) (*RedisQueue, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Redis URL: %w", err)
	}

	client := redis.NewClient(opts)

	// Test the connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := client.Ping(ctx).Result(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	// Initialize the consumer group if it doesn't exist
	// The "$" means to start consuming from the newest message after creation
	// Use "0" to consume all messages in the stream
	err = client.XGroupCreateMkStream(ctx, MessageStreamKey, ConsumerGroup, "$").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		return nil, fmt.Errorf("failed to create consumer group: %w", err)
	}

	return &RedisQueue{client: client}, nil
}

// Close closes the Redis connection
func (q *RedisQueue) Close() error {
	return q.client.Close()
}

// EnqueueMessage adds a message to the Redis Stream
func (q *RedisQueue) EnqueueMessage(ctx context.Context, message *models.Message) error {
	// Validate message
	if message.SenderID == "" || message.ReceiverID == "" {
		return fmt.Errorf("sender and receiver IDs are required")
	}
	
	// Store the original message ID if it exists
	originalID := message.ID
	
	// Serialize the message to JSON
	messageJSON, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Add the message to the Redis Stream
	values := map[string]interface{}{
		"message": string(messageJSON),
		"original_id": originalID, // Store the original UUID for reference
	}
	
	// Add to stream and get the message ID
	msgID, err := q.client.XAdd(ctx, &redis.XAddArgs{
		Stream: MessageStreamKey,
		ID:     "*", // Auto-generate ID
		Values: values,
	}).Result()
	
	if err != nil {
		return fmt.Errorf("failed to add message to stream: %w", err)
	}
	
	// Store the Redis Stream message ID in the RedisStreamID field for reference
	// This is the ID that should be used for acknowledgment
	message.RedisStreamID = msgID

	return nil
}

// DequeueMessage consumes a message from the Redis Stream
func (q *RedisQueue) DequeueMessage(ctx context.Context) (*models.Message, error) {
	// Read from the stream with a consumer group
	// This allows for message acknowledgment and prevents message loss
	streams, err := q.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    ConsumerGroup,
		Consumer: ConsumerName,
		Streams:  []string{MessageStreamKey, ">"},  // ">" means only new messages
		Count:    1,                                // Process one message at a time
		Block:    0,                                // Block until a message is available
	}).Result()
	
	if err != nil {
		return nil, fmt.Errorf("failed to read from stream: %w", err)
	}
	
	// No messages available
	if len(streams) == 0 || len(streams[0].Messages) == 0 {
		return nil, fmt.Errorf("no messages available")
	}
	
	// Get the first message
	xMessage := streams[0].Messages[0]
	redisStreamID := xMessage.ID
	
	// Extract the message JSON from the values
	messageJSON, ok := xMessage.Values["message"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid message format")
	}
	
	// Deserialize the message
	var message models.Message
	if err := json.Unmarshal([]byte(messageJSON), &message); err != nil {
		return nil, fmt.Errorf("failed to unmarshal message: %w", err)
	}
	
	// Check if there's an original ID in the message metadata
	if originalID, ok := xMessage.Values["original_id"].(string); ok && originalID != "" {
		// Keep the original UUID as the message ID
		message.ID = originalID
	}

	// Store the Redis Stream message ID in the RedisStreamID field for acknowledgment
	message.RedisStreamID = redisStreamID
	
	return &message, nil
}

// AcknowledgeMessage marks a message as processed in Redis Streams
func (q *RedisQueue) AcknowledgeMessage(ctx context.Context, messageID string) error {
	// Validate the message ID format
	if messageID == "" {
		return fmt.Errorf("empty message ID provided for acknowledgment")
	}

	// Acknowledge the message in the consumer group
	err := q.client.XAck(ctx, MessageStreamKey, ConsumerGroup, messageID).Err()
	if err != nil {
		return fmt.Errorf("failed to acknowledge message: %w", err)
	}
	
	return nil
}

// CleanupPendingEntries handles the PEL (Pending Entries List) limit issue by claiming and acknowledging
// any pending messages that might be causing the limit to be reached
func (q *RedisQueue) CleanupPendingEntries(ctx context.Context) error {
	// Get all pending messages for the consumer group
	pendingInfo, err := q.client.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream:   MessageStreamKey,
		Group:    ConsumerGroup,
		Start:    "-", // Start from the earliest pending message
		End:      "+", // End at the latest pending message
		Count:    100,    // Process in batches of 100
		Consumer: ConsumerName,
	}).Result()

	if err != nil {
		return fmt.Errorf("failed to get pending messages: %w", err)
	}

	// If there are no pending messages, we're done
	if len(pendingInfo) == 0 {
		return nil
	}

	// Collect message IDs to claim and acknowledge
	messageIDs := make([]string, 0, len(pendingInfo))
	for _, pending := range pendingInfo {
		messageIDs = append(messageIDs, pending.ID)
	}

	// Claim the messages to this consumer
	// This is necessary before we can acknowledge them
	_, err = q.client.XClaim(ctx, &redis.XClaimArgs{
		Stream:   MessageStreamKey,
		Group:    ConsumerGroup,
		Consumer: ConsumerName,
		MinIdle:  time.Minute, // Claim messages that have been idle for at least 1 minute
		Messages: messageIDs,
	}).Result()

	if err != nil {
		return fmt.Errorf("failed to claim pending messages: %w", err)
	}

	// Acknowledge the claimed messages
	err = q.client.XAck(ctx, MessageStreamKey, ConsumerGroup, messageIDs...).Err()
	if err != nil {
		return fmt.Errorf("failed to acknowledge claimed messages: %w", err)
	}

	fmt.Printf("Cleaned up %d pending messages from Redis Stream\n", len(messageIDs))

	// If we processed a full batch, there might be more pending messages
	// Recursively call this function to process the next batch
	if len(pendingInfo) == 100 {
		return q.CleanupPendingEntries(ctx)
	}

	return nil
}
