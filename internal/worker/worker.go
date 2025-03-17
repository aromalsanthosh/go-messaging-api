package worker

import (
	"context"
	"log"
	"time"

	"github.com/aromalsanthosh/go-messaging-api/internal/db"
	"github.com/aromalsanthosh/go-messaging-api/internal/queue"
)

// WorkerContext holds the context and cancel function for the worker
type WorkerContext struct {
	Ctx    context.Context
	Cancel context.CancelFunc
}

// StartWorker starts a background worker to process messages from the Redis Stream
func StartWorker(database *db.Database, messageQueue *queue.RedisQueue) *WorkerContext {
	ctx, cancel := context.WithCancel(context.Background())
	workerCtx := &WorkerContext{
		Ctx:    ctx,
		Cancel: cancel,
	}

	// Start a goroutine to periodically maintain Redis Stream health
	go func() {
		// cleanup frequency  every 10 minutes
		cleanupTicker := time.NewTicker(10 * time.Minute)
		// Add stream trimming every 30 minutes
		trimTicker := time.NewTicker(30 * time.Minute)
		defer cleanupTicker.Stop()
		defer trimTicker.Stop()

		// Define the maximum number of messages to keep in the stream
		const maxStreamLength int64 = 10000

		for {
			select {
			case <-ctx.Done():
				return
			case <-cleanupTicker.C:
				// Clean up pending entries that might be causing the PEL limit issue
				if err := messageQueue.CleanupPendingEntries(ctx); err != nil {
					log.Printf("Error cleaning up pending entries: %v", err)
				}
			case <-trimTicker.C:
				// Trim the stream to prevent unbounded growth
				if err := messageQueue.TrimStream(ctx, maxStreamLength); err != nil {
					log.Printf("Error trimming Redis Stream: %v", err)
				}
			}
		}
	}()

	go func() {
		log.Println("Message worker started")
		for {
			select {
			case <-ctx.Done():
				log.Println("Message worker stopped")
				return
			default:
				// Process messages from the Redis Stream
				message, err := messageQueue.DequeueMessage(ctx)
				if err != nil {
					// If the context was canceled, exit gracefully
					if ctx.Err() != nil {
						return
					}
					
					// Log error but don't log 'no messages available' as it's normal
					if err.Error() != "no messages available" {
						log.Printf("Error dequeuing message: %v", err)
					}
					
					// Add a small delay to avoid hammering Redis in case of persistent errors
					time.Sleep(1 * time.Second)
					continue
				}

				// Save the message to the database
				if err := database.SaveMessage(ctx, message); err != nil {
					log.Printf("Error saving message: %v", err)
					continue
				}
				
				// Acknowledge the message in Redis Stream after successful processing
				// This is a key feature of Redis Streams - messages are only acknowledged after successful processing
				if err := messageQueue.AcknowledgeMessage(ctx, message.RedisStreamID); err != nil {
					log.Printf("Warning: Failed to acknowledge message %s: %v", message.ID, err)
					// Continue processing even if acknowledgment fails
				}

				log.Printf("Processed message from %s to %s (ID: %s)", message.SenderID, message.ReceiverID, message.ID)
			}
		}
	}()

	return workerCtx
}

// StopWorker stops the background worker
func StopWorker(workerCtx *WorkerContext) {
	if workerCtx != nil && workerCtx.Cancel != nil {
		workerCtx.Cancel()
	}
}
