package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
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

	// Add the message to the Redis Stream with automatic trimming
	// This prevents unbounded memory growth in Redis
	values := map[string]interface{}{
		"message": string(messageJSON),
		"original_id": originalID, // Store the original UUID for reference
	}
	
	// Use a pipeline to reduce network round-trips
	pipe := q.client.Pipeline()
	
	// Add to stream with MAXLEN trimming to keep the stream at a reasonable size
	// The ~ makes it an approximate trimming which is much more efficient
	xAddCmd := pipe.XAdd(ctx, &redis.XAddArgs{
		Stream: MessageStreamKey,
		ID:     "*", // Auto-generate ID
		Values: values,
		MaxLen: 10000,    // Keep approximately 10,000 messages
		Approx: true,     // Use approximate trimming for better performance
	})
	
	// Execute pipeline
	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to add message to stream: %w", err)
	}
	
	// Get the message ID from the XAdd command
	msgID, err := xAddCmd.Result()
	if err != nil {
		return fmt.Errorf("failed to get message ID: %w", err)
	}
	
	// Store the Redis Stream message ID in the RedisStreamID field for reference
	// This is the ID that should be used for acknowledgment
	message.RedisStreamID = msgID

	return nil
}

// DequeueMessage consumes a message from the Redis Stream
func (q *RedisQueue) DequeueMessage(ctx context.Context) (*models.Message, error) {
	// First try to read pending messages (messages that were delivered but not acknowledged)
	// This helps prevent message loss in case of crashes
	pendingStreams, pendingErr := q.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    ConsumerGroup,
		Consumer: ConsumerName,
		Streams:  []string{MessageStreamKey, "0"}, // "0" means read pending messages first
		Count:    1,                               // Process one message at a time
		Block:    100 * time.Millisecond,          // Short timeout for pending messages
	}).Result()
	
	// If we found pending messages, use them
	var streams []redis.XStream
	var err error
	
	if pendingErr == nil && len(pendingStreams) > 0 && len(pendingStreams[0].Messages) > 0 {
		streams = pendingStreams
	} else {
		// Otherwise, read new messages from the stream
		streams, err = q.client.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    ConsumerGroup,
			Consumer: ConsumerName,
			Streams:  []string{MessageStreamKey, ">"}, // ">" means only new messages
			Count:    1,                               // Process one message at a time
			Block:    1 * time.Second,                 // Block with timeout for new messages
		}).Result()
	}

	if err != nil {
		// If no messages available, return a specific error
		if err == redis.Nil {
			return nil, fmt.Errorf("no messages available")
		}
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
		// Acknowledge invalid messages to prevent them from blocking the queue
		_ = q.client.XAck(ctx, MessageStreamKey, ConsumerGroup, redisStreamID).Err()
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
	// Use a more efficient approach with a single loop instead of recursion
	totalCleaned := 0
	batchSize := 100
	
	for {
		// Get pending messages for the consumer group in batches
		pendingInfo, err := q.client.XPendingExt(ctx, &redis.XPendingExtArgs{
			Stream:   MessageStreamKey,
			Group:    ConsumerGroup,
			Start:    "-", // Start from the earliest pending message
			End:      "+", // End at the latest pending message
			Count:    int64(batchSize),
			Consumer: ConsumerName,
		}).Result()

		// Handle errors but don't fail the entire operation for non-critical errors
		if err != nil {
			// If the stream doesn't exist yet, that's not an error
			if err == redis.Nil || strings.Contains(err.Error(), "NOGROUP") {
				return nil
			}
			return fmt.Errorf("failed to get pending messages: %w", err)
		}

		// If there are no pending messages, we're done
		if len(pendingInfo) == 0 {
			break
		}

		// Collect message IDs to claim and acknowledge
		messageIDs := make([]string, 0, len(pendingInfo))
		for _, pending := range pendingInfo {
			// Only process messages that have been idle for some time (> 5 minutes)
			if pending.Idle > time.Minute*5 {
				messageIDs = append(messageIDs, pending.ID)
			}
		}

		// If no messages need processing after filtering, we're done with this batch
		if len(messageIDs) == 0 {
			if len(pendingInfo) < batchSize {
				break // No more messages to process
			}
			continue // Check next batch
		}

		// Claim and acknowledge in a single batch to reduce Redis calls
		_, err = q.client.XClaim(ctx, &redis.XClaimArgs{
			Stream:   MessageStreamKey,
			Group:    ConsumerGroup,
			Consumer: ConsumerName,
			MinIdle:  time.Minute*5, // Only claim messages idle for at least 5 minutes
			Messages: messageIDs,
		}).Result()

		if err != nil && err != redis.Nil {
			// Log the error but continue processing
			fmt.Printf("Warning: failed to claim pending messages: %v\n", err)
			// If we can't claim, don't try to acknowledge
			continue
		}

		// Acknowledge the claimed messages
		if len(messageIDs) > 0 {
			err = q.client.XAck(ctx, MessageStreamKey, ConsumerGroup, messageIDs...).Err()
			if err != nil {
				fmt.Printf("Warning: failed to acknowledge claimed messages: %v\n", err)
			} else {
				totalCleaned += len(messageIDs)
			}
		}

		// If we processed less than a full batch, we're done
		if len(pendingInfo) < batchSize {
			break
		}
	}

	if totalCleaned > 0 {
		fmt.Printf("Cleaned up %d pending messages from Redis Stream\n", totalCleaned)
	}

	return nil
}
