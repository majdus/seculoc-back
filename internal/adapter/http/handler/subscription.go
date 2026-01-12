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

// Subscribe godoc
// @Summary      Subscribe to a plan
// @Description  Subscribe user to Discovery, Serenity, or Premium plan
// @Tags         subscriptions
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body map[string]interface{} true "Subscription Info"
// @Success      201  {object}  map[string]interface{}
// @Failure      400  {object}  map[string]string
// @Router       /subscriptions [post]
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

// IncreaseLimit godoc
// @Summary      Increase property limit
// @Description  Purchase additional property slots (Serenity/Premium only)
// @Tags         subscriptions
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body map[string]interface{} true "Limit Info"
// @Success      200  {object}  map[string]interface{}
// @Failure      400  {object}  map[string]string
// @Router       /subscriptions/upgrade [post]
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
