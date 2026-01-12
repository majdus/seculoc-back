package handler

import (
	"net/http"

	"seculoc-back/internal/adapter/http/middleware"
	"seculoc-back/internal/core/service"

	"github.com/gin-gonic/gin"
)

type SubscriptionHandler struct {
	svc *service.SubscriptionService
}

func NewSubscriptionHandler(svc *service.SubscriptionService) *SubscriptionHandler {
	return &SubscriptionHandler{svc: svc}
}

type SubscribeRequest struct {
	Plan      string `json:"plan" binding:"required,oneof=discovery serenity premium"`
	Frequency string `json:"frequency" binding:"required,oneof=monthly yearly"`
}

func (h *SubscriptionHandler) Subscribe(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req SubscribeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.svc.SubscribeUser(c.Request.Context(), userID, req.Plan, req.Frequency)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusCreated)
}

type UpgradeLimitRequest struct {
	AdditionalSlots int32 `json:"additional_slots" binding:"required,min=1"`
}

func (h *SubscriptionHandler) IncreaseLimit(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req UpgradeLimitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.svc.IncreaseLimit(c.Request.Context(), userID, req.AdditionalSlots)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusOK)
}
