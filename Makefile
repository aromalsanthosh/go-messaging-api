# Run the application
run:
	go run cmd/api/main.go

# Build & Start Docker containers
docker-up:
	docker compose up -d

# Stop Docker containers
docker-down:
	docker compose down

# Start Docker and run the application
dev: docker-up
	go run cmd/api/main.go

# Clean up all Docker containers
docker-clean:
	docker stop $$(docker ps -aq) || true
	docker rm $$(docker ps -aq) || true