package main

import (
	"ProjectMessageService/config"
	"ProjectMessageService/internal/handler"
	"ProjectMessageService/internal/repository"
	"ProjectMessageService/internal/service"
	"context"

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

	messageTypes := []string{"message", "ping"} // Список типов сообщений

	if err = repository.RunMigrations(db, messageTypes); err != nil {
		app.Log.Fatalf("Не удалось выполнить миграцию: %v", err)
	}

	repo := repository.NewRepository(db, app)
	kafkaWriter := service.NewKafkaWriter(cfg)

	messageService := service.NewMessageService(repo, kafkaWriter, app)

	newHandler := handler.NewHandler(messageService)

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.POST("/messages", newHandler.CreateMessage)

	go messageService.ConsumeMessages(context.Background(), cfg, messageTypes)

	r.GET("/stats", newHandler.GetStats)

	if err = r.Run(":8080"); err != nil {
		app.Log.Fatalf("Не удалось запустить сервер: %v", err)
	}
}
