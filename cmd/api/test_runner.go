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

// TestAPI demonstrates the API functionality by sending 100 messages
func TestAPI() {
	baseURL := "http://localhost:8080/api/v1"

	// Generate random usernames with different prefixes
	userPrefixes := []string{"user", "customer", "client", "member", "account"}
	user1 := userPrefixes[rand.Intn(len(userPrefixes))] + "_" + randomString(6)
	user2 := userPrefixes[rand.Intn(len(userPrefixes))] + "_" + randomString(6)
	fmt.Printf("Using randomly generated users: %s and %s\n", user1, user2)

	// Wait for the server to start
	fmt.Println("Waiting for server to start...")
	time.Sleep(2 * time.Second)

	rand.Seed(time.Now().UnixNano())

	// Insert 100 random messages between user1 and user2
	fmt.Println("\nInserting 100 random messages...")
	messageIDs := make([]string, 0, 100)
	
	for i := 0; i < 100; i++ {
		sender := user1
		receiver := user2
		if rand.Intn(2) == 1 {
			sender, receiver = receiver, sender
		}

		message := TestMessage{
			SenderID:   sender,
			ReceiverID: receiver,
			Content:    fmt.Sprintf("Test message %d: %s", i+1, randomString(20)),
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
		}

		// Small delay to ensure messages have different timestamps
		time.Sleep(50 * time.Millisecond)
	}

	fmt.Printf("Successfully inserted %d messages\n", len(messageIDs))

	fmt.Println("Waiting for messages to be processed...")
	time.Sleep(3 * time.Second)

	fmt.Println("\nTest completed - 100 messages sent to Redis Stream")
}

func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

func runTest() {
	fmt.Println("Starting API test - sending 100 messages...")
	TestAPI()
	fmt.Println("\nAPI test completed!")
}

func main() {
	runTest()
}
