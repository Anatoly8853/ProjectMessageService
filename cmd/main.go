package main

import (
	"ProjectMessageService/config"
	"ProjectMessageService/internal/handler"
	"ProjectMessageService/internal/repository"
	"ProjectMessageService/internal/service"
	`context`

	"github.com/gin-gonic/gin"
)

func main() {

	// Настраиваем логгер
	app := config.SetupApplication()
	cfg := config.LoadConfig(app)

	db, err := repository.NewPostgresDB(cfg, 5)
	if err != nil {
		app.Log.Fatalf("Не удалось подключиться к базе данных: %v", err)
	}

	if err = repository.RunMigrations(db, config.MessageTypes); err != nil {
		app.Log.Fatalf("Не удалось выполнить миграцию: %v", err)
	}

	repo := repository.NewRepository(db, app)
	kafkaWriter := service.NewKafkaWriter(cfg)

	messageService := service.NewMessageService(repo, kafkaWriter, app)
	newHandler := handler.NewHandler(cfg, messageService, repo, app)

	go messageService.ConsumeMessages(context.Background(), cfg, config.MessageTypes)

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.POST("/users", newHandler.CreateUser)
	r.POST("/users/login", newHandler.LoginUser)
	r.POST("/token/renew_access", newHandler.RenewAccessToken)

	authRoutes := r.Group("/").Use(handler.AuthMiddleware(newHandler.TokenMaker))
	authRoutes.POST("/messages", newHandler.CreateMessage)
	authRoutes.GET("/stats", newHandler.GetStats)
	if err = r.Run(":8080"); err != nil {
		app.Log.Fatalf("Не удалось запустить сервер: %v", err)
	}
}
