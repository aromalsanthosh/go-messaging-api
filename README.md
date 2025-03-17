
# Go Messaging API

A real-time messaging platform backend API built with Go, featuring asynchronous message processing with Redis queue.

  ![go-zoko](https://github.com/user-attachments/assets/b9a7e892-c08d-43db-b66a-e883bb617df3)


## Features

  

- Send text messages asynchronously using a message queue

- Retrieve conversation history between users with efficient offset-based pagination

- Mark messages as read

- Queue-based message processing with Redis

- Optimized database indexes for fast query performance

  

## Tech Stack

  

-  **Backend Language**: Go

-  **API Framework**: Gin

-  **Database**: PostgreSQL

-  **Message Queue**: Redis

  

## API Endpoints

  

### Send a Message (Asynchronous with Queue)

  

```

POST /api/v1/messages

```

  

Request Body:

```json

{

"sender_id": "user123",

"receiver_id": "user456",

"content": "Hello, how are you?"

}

```

  

Response:

```json

{

"message": "Message queued for delivery",

"message_id": "7644316f-c6ca-47ad-af72-4d7ce36363ba"

}

```

  

### Retrieve Conversation History (with Pagination)

  

```

GET /api/v1/messages?user1=user123&user2=user456&limit=20&offset=0

```

  

Pagination Parameters:

  

-  `limit`: Maximum number of messages to return (default: 20)

-  `offset`: Number of messages to skip (default: 0)

  

Response:

```json

{

"data": [

{

"message_id": "7644316f-c6ca-47ad-af72-4d7ce36363ba",

"sender_id": "user123",

"receiver_id": "user456",

"content": "Hello, how are you?",

"timestamp": "2025-03-15T16:14:25.993674Z",

"read": false

}

],

"pagination": {

"total_count": 42,

"has_more": true,

"offset": 0,

"limit": 20

}

}

```

  

### Mark a Message as Read

  

```

PATCH /api/v1/messages/{message_id}/read

```

  

Response:

```json

{

"status": "read"

}

```

  

## Deployment Options

  

### Local Development

  

Run the application locally with Docker containers for infrastructure:

  

#### Prerequisites

  

- Go 1.16 or higher

- Docker and Docker Compose

  

### Step 1: Clone the Repository

  

```bash

git  clone  https://github.com/aromalsanthosh/go-messaging-api.git

cd  go-messaging-api

```

  

### Step 2: Start the Database and Redis with Docker Compose

  

```bash

docker  compose  up  -d  postgres  redis

```

  

This will start PostgreSQL on port 5432 and Redis on port 6379.

  

### Step 3: Configure Environment Variables

  

Create a `.env` file in the root directory with the following content:

  

```
# API Server Configuration
PORT=8080

# Database Configuration
NEON_DB_URL=postgresql://postgres:postgres@localhost:5432/messaging?sslmode=disable

# Redis Configuration
REDIS_URL=redis://localhost:6379
```

  

### Step 4: Install Dependencies

  

```bash

go  mod  download

```

  

### Step 5: Run the Application

  

```bash

go  run  cmd/api/main.go

```

  

The server will start on port 8080 and the message worker will begin processing messages from the queue.

  

test using curl:

  

```bash

# Send a message

curl  -X  POST  -H  "Content-Type: application/json"  -d  '{"sender_id": "user123", "receiver_id": "user456", "content": "Hello, how are you?"}'  http://localhost:8080/api/v1/messages

  

# Get conversation history (default pagination - 20 messages)

curl  -X  GET  "http://localhost:8080/api/v1/messages?user1=user123&user2=user456"

  

# Get conversation with custom limit

curl  -X  GET  "http://localhost:8080/api/v1/messages?user1=user123&user2=user456&limit=5"

  

# Get conversation with offset and limit for pagination

curl  -X  GET  "http://localhost:8080/api/v1/messages?user1=user123&user2=user456&offset=20&limit=10"

  

# Mark a message as read (replace with actual message ID)

curl  -X  PATCH  http://localhost:8080/api/v1/messages/{message_id}/read

```
  

## Environment Variables

  

-  `PORT` - API server port (default: 8080)

-  `NEON_DB_URL` - PostgreSQL connection string

-  `REDIS_URL` - Redis connection string

## Test Files

#### To run test_runner.go that sends random 100 messages
```bash
go run -tags=test cmd/api/test_runner.go
```

#### To run test_runner2.go that sends 30 messages with 1 second delay between each message
```bash
go run -tags=test cmd/api/test_runner2.go
```
