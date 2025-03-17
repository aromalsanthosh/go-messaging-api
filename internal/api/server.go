package api

import (
	"github.com/aromalsanthosh/go-messaging-api/internal/config"
	"github.com/aromalsanthosh/go-messaging-api/internal/db"
	"github.com/aromalsanthosh/go-messaging-api/internal/queue"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

type Server struct {
	router      *gin.Engine
	config      *config.Config
	database    *db.Database
	messageQueue *queue.RedisQueue
}

func NewServer(cfg *config.Config, database *db.Database, messageQueue *queue.RedisQueue) *Server {
	router := gin.Default()

	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	server := &Server{
		router:      router,
		config:      cfg,
		database:    database,
		messageQueue: messageQueue,
	}

	SetupRoutes(router, database, messageQueue)

	return server
}

func (s *Server) Start(addr string) error {
	return s.router.Run(addr)
}
