package handler

import (
	`ProjectMessageService/config`
	`ProjectMessageService/internal/api`
	`ProjectMessageService/internal/repository`
	"ProjectMessageService/internal/service"
	`ProjectMessageService/internal/token`
	"ProjectMessageService/internal/utils"
	`ProjectMessageService/util`
	"context"
	`errors`
	"net/http"

	"github.com/gin-gonic/gin"
	`github.com/gin-gonic/gin/binding`
	`github.com/go-playground/validator/v10`
)

type Handler struct {
	config     config.Config
	service    *service.MessageService
	TokenMaker token.Maker
	app        *config.Application
	repo       *repository.Repository
}

func NewHandler(config config.Config, service *service.MessageService, repo *repository.Repository, app *config.Application) *Handler {
	tokenMaker, err := token.NewPasetoMaker(config.TokenSymmetricKey)
	if err != nil {
		app.Log.Errorf("cannot create token maker: %v", err)
		return nil
	}

	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		_ = v.RegisterValidation("topic", validCurrency)
	}

	return &Handler{config: config, service: service, TokenMaker: tokenMaker, repo: repo, app: app}
}

func (h *Handler) CreateMessage(c *gin.Context) {
	var input utils.Message

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(err))
		return
	}

	if err := h.service.SaveMessage(context.Background(), input); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(err))
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "message received"})
}

func (h *Handler) GetStats(c *gin.Context) {
	var topic utils.Messages

	if err := c.ShouldBindJSON(&topic); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(err))
		return
	}

	count, err := h.service.GetStats(context.Background(), topic)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(err))
		return
	}

	c.JSON(http.StatusOK, gin.H{"processed_messages": count})
}

func (h *Handler) CreateUser(ctx *gin.Context) {
	var req api.CreateUserRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, ErrorResponse(err))
		return
	}

	hashedPassword, err := util.HashPassword(req.Password)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, ErrorResponse(err))
		return
	}

	arg := repository.CreateUserParams{
		Username:       req.Username,
		HashedPassword: hashedPassword,
		FullName:       req.FullName,
		Email:          req.Email,
	}

	user, err := h.repo.CreateUser(ctx, arg)
	if err != nil {
		if util.ErrorCode(err) == util.UniqueViolation {
			ctx.JSON(http.StatusForbidden, ErrorResponse(err))
			return
		}
		ctx.JSON(http.StatusInternalServerError, ErrorResponse(err))
		return
	}

	rsp := api.NewUserResponse(user)
	ctx.JSON(http.StatusOK, rsp)
}

func ErrorResponse(err error) gin.H {
	return gin.H{"error": err.Error()}
}

func (h *Handler) LoginUser(ctx *gin.Context) {
	var req api.LoginUserRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, ErrorResponse(err))
		return
	}

	user, err := h.repo.GetUser(ctx, req.Username)

	if err != nil {
		if errors.Is(err, util.ErrRecordNotFound) {
			ctx.JSON(http.StatusNotFound, ErrorResponse(err))
			return
		}
		ctx.JSON(http.StatusInternalServerError, ErrorResponse(err))
		return
	}

	err = util.CheckPassword(req.Password, user.HashedPassword)
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, ErrorResponse(err))
		return
	}

	accessToken, accessPayload, err := h.TokenMaker.CreateToken(
		user.Username,
		user.Role,
		h.config.AccessTokenDuration,
	)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, ErrorResponse(err))
		return
	}

	refreshToken, refreshPayload, err := h.TokenMaker.CreateToken(
		user.Username,
		user.Role,
		h.config.RefreshTokenDuration,
	)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, ErrorResponse(err))
	}

	session, err := h.repo.CreateSession(ctx, repository.CreateSessionParams{
		ID:           refreshPayload.ID,
		Username:     user.Username,
		RefreshToken: refreshToken,
		UserAgent:    ctx.Request.UserAgent(),
		ClientIp:     ctx.ClientIP(),
		IsBlocked:    false,
		ExpiresAt:    refreshPayload.ExpiredAt,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, ErrorResponse(err))
	}

	rsp := api.LoginUserResponse{
		SessionID:             session.ID,
		AccessToken:           accessToken,
		AccessTokenExpiresAt:  accessPayload.ExpiredAt,
		RefreshToken:          refreshToken,
		RefreshTokenExpiresAt: refreshPayload.ExpiredAt,
		User:                  api.NewUserResponse(user),
	}
	ctx.JSON(http.StatusOK, rsp)
}
