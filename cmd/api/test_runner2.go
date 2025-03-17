//go:build test

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"time"
)

type TestMessage struct {
	SenderID   string `json:"sender_id"`
	ReceiverID string `json:"receiver_id"`
	Content    string `json:"content"`
}

// TestAPIWithDelay demonstrates the API functionality by sending 30 messages with a 1-second delay
func TestAPIWithDelay() {
	baseURL := "http://localhost:8080/api/v1"

	user1 := "aromal"
	user2 := "zoko"
	fmt.Printf("Using specific users: %s and %s\n", user1, user2)

	fmt.Println("Waiting for server to start...")
	time.Sleep(2 * time.Second)

	rand.Seed(time.Now().UnixNano())

	// Insert 30 messages between aromal and zoko with 1-second delay
	fmt.Println("\nInserting 30 messages with 1-second delay between each...")
	messageIDs := make([]string, 0, 30)

	for i := 0; i < 30; i++ {
		// Alternate sender and receiver
		sender := user1
		receiver := user2
		if i%2 == 1 {
			sender, receiver = receiver, sender
		}

		timestamp := time.Now().Format(time.RFC3339)
		message := TestMessage{
			SenderID:   sender,
			ReceiverID: receiver,
			Content:    fmt.Sprintf("Message %d sent at %s: %s", i+1, timestamp, generateRandomContent()),
		}

		// Send message
		messageJSON, _ := json.Marshal(message)
		resp, err := http.Post(baseURL+"/messages", "application/json", bytes.NewBuffer(messageJSON))
		if err != nil {
			fmt.Printf("Error sending message %d: %v\n", i+1, err)
			continue
		}

		var sendResponse map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&sendResponse)
		resp.Body.Close()

		if messageID, ok := sendResponse["message_id"].(string); ok {
			messageIDs = append(messageIDs, messageID)
			fmt.Printf("Message %d sent: %s -> %s\n", i+1, sender, receiver)
		}

		// 1-second delay between messages
		time.Sleep(1 * time.Second)
	}

	fmt.Printf("Successfully inserted %d messages\n", len(messageIDs))

	fmt.Println("Waiting for messages to be processed...")
	time.Sleep(3 * time.Second)

	fmt.Println("\nRetrieving conversation history...")
	resp, err := http.Get(fmt.Sprintf("%s/messages?user1=%s&user2=%s&limit=30", baseURL, user1, user2))
	if err != nil {
		fmt.Printf("Error retrieving conversation: %v\n", err)
	} else {
		var conversation map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&conversation)
		resp.Body.Close()

		if data, ok := conversation["data"].([]interface{}); ok {
			fmt.Printf("Retrieved %d messages from conversation\n", len(data))
		}
	}

	fmt.Println("\nTest completed - 30 messages sent with 1-second delay")
}

func generateRandomContent() string {
	phrases := []string{
		"How's your day going?",
		"Just checking in!",
		"Let's meet up soon.",
		"Did you see the latest update?",
		"I'm working on the project now.",
		"Can you review my code?",
		"The API is working great!",
		"Redis Streams are awesome.",
		"PostgreSQL performance is excellent.",
		"I fixed that bug we discussed.",
	}
	return phrases[rand.Intn(len(phrases))]
}

func main() {
	fmt.Println("Starting API test with delay - sending 30 messages between aromal and zoko...")
	TestAPIWithDelay()
	fmt.Println("\nAPI test with delay completed!")
}
