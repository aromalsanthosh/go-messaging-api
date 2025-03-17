package api

import (
	"github.com/aromalsanthosh/go-messaging-api/internal/api/handlers"
	"github.com/aromalsanthosh/go-messaging-api/internal/db"
	"github.com/aromalsanthosh/go-messaging-api/internal/queue"
	"github.com/gin-gonic/gin"
)

// SetupRoutes configures the API routes
func SetupRoutes(router *gin.Engine, database *db.Database, messageQueue *queue.RedisQueue) {
	messageHandler := handlers.NewMessageHandler(database, messageQueue)

	// API v1 group
	v1 := router.Group("/api/v1")
	{
		// Messages endpoints
		messages := v1.Group("/messages")
		{
			// Send a message (POST /api/v1/messages)
			messages.POST("", messageHandler.SendMessage)

			// Get conversation history (GET /api/v1/messages?user1=user123&user2=user456)
			messages.GET("", messageHandler.GetConversation)

			// Mark a message as read (PATCH /api/v1/messages/:id/read)
			messages.PATCH(":id/read", messageHandler.MarkAsRead)
		}
	}

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "ok",
		})
	})
}
