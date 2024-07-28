package handler

import (
	"ProjectMessageService/internal/service"
	"ProjectMessageService/internal/utils"
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *service.MessageService
}

func NewHandler(service *service.MessageService) *Handler {
	return &Handler{service: service}
}

func (h *Handler) CreateMessage(c *gin.Context) {
	var input utils.Message

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.SaveMessage(context.Background(), input); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "message received"})
}

func (h *Handler) GetStats(c *gin.Context) {
	var topic utils.Messages

	if err := c.ShouldBindJSON(&topic); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	count, err := h.service.GetStats(context.Background(), topic)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"processed_messages": count})
}
