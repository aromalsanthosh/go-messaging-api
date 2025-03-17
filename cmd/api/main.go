package main

import (
	"fmt"
	"log"
	"os"

	"github.com/aromalsanthosh/go-messaging-api/internal/api"
	"github.com/aromalsanthosh/go-messaging-api/internal/config"
	"github.com/aromalsanthosh/go-messaging-api/internal/db"
	"github.com/aromalsanthosh/go-messaging-api/internal/queue"
	"github.com/aromalsanthosh/go-messaging-api/internal/worker"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("Warning: Error loading .env file:", err)
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	database, err := db.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()

	messageQueue, err := queue.NewRedisQueue(cfg.RedisURL)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer messageQueue.Close()

	workerCtx := worker.StartWorker(database, messageQueue)
	defer worker.StopWorker(workerCtx)

	server := api.NewServer(cfg, database, messageQueue)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("Server starting on port %s...\n", port)
	if err := server.Start(fmt.Sprintf(":%s", port)); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
