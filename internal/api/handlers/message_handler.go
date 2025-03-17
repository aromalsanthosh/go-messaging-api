package handlers

import (
	"log"
	"net/http"
	"strconv"

	"github.com/aromalsanthosh/go-messaging-api/internal/db"
	"github.com/aromalsanthosh/go-messaging-api/internal/models"
	"github.com/aromalsanthosh/go-messaging-api/internal/queue"
	"github.com/gin-gonic/gin"
)

type MessageHandler struct {
	database     *db.Database
	messageQueue *queue.RedisQueue
}

func NewMessageHandler(database *db.Database, messageQueue *queue.RedisQueue) *MessageHandler {
	return &MessageHandler{
		database:     database,
		messageQueue: messageQueue,
	}
}

// SendMessage handles the POST /messages endpoint
func (h *MessageHandler) SendMessage(c *gin.Context) {
	var req models.MessageRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	message := models.NewMessage(req.SenderID, req.ReceiverID, req.Content)

	// Enqueue the message for asynchronous processing
	if err := h.messageQueue.EnqueueMessage(c.Request.Context(), message); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to enqueue message: " + err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"message": "Message queued for delivery",
		"message_id": message.ID,
	})
}

// GetConversation handles the GET /messages endpoint
func (h *MessageHandler) GetConversation(c *gin.Context) {
	user1 := c.Query("user1")
	user2 := c.Query("user2")

	if user1 == "" || user2 == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Both user1 and user2 query parameters are required"})
		return
	}

	paginationParams := parsePaginationParams(c)

	// Get the conversation history with pagination
	messages, err := h.database.GetConversation(c.Request.Context(), user1, user2, paginationParams)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve conversation: " + err.Error()})
		return
	}

	// Get total count for pagination metadata
	totalCount, err := h.database.GetConversationCount(c.Request.Context(), user1, user2)
	if err != nil {
		log.Printf("Error getting conversation count: %v", err)
	}

	hasMore := len(messages) >= paginationParams.Limit && (paginationParams.Offset + len(messages)) < totalCount

	// Create the paginated response
	response := models.PaginatedResponse{
		Data: messages,
		Pagination: models.PaginationMeta{
			TotalCount: totalCount,
			HasMore:    hasMore,
			Offset:     paginationParams.Offset,
			Limit:      paginationParams.Limit,
		},
	}

	c.JSON(http.StatusOK, response)
}

func parsePaginationParams(c *gin.Context) models.PaginationParams {
	params := models.PaginationParams{}

	if limitStr := c.Query("limit"); limitStr != "" {
		limit, err := strconv.Atoi(limitStr)
		if err == nil && limit > 0 {
			params.Limit = limit
		}
	}

	if offsetStr := c.Query("offset"); offsetStr != "" {
		offset, err := strconv.Atoi(offsetStr)
		if err == nil && offset >= 0 {
			params.Offset = offset
		}
	}

	return params
}



// MarkAsRead handles the PATCH /messages/:id/read endpoint
func (h *MessageHandler) MarkAsRead(c *gin.Context) {
	messageID := c.Param("id")
	if messageID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Message ID is required"})
		return
	}

	// Mark the message as read
	if err := h.database.MarkMessageAsRead(c.Request.Context(), messageID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to mark message as read: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, models.ReadStatus{Status: "read"})
}
