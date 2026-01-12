package handler

import (
	"net/http"

	"seculoc-back/internal/adapter/http/middleware"
	"seculoc-back/internal/core/service"
	"seculoc-back/internal/platform/logger"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type SubscriptionHandler struct {
	svc     *service.SubscriptionService
	userSvc *service.UserService
}

func NewSubscriptionHandler(svc *service.SubscriptionService, userSvc *service.UserService) *SubscriptionHandler {
	return &SubscriptionHandler{svc: svc, userSvc: userSvc}
}

type SubscribeRequest struct {
	Plan      string `json:"plan" binding:"required,oneof=discovery serenity premium"`
	Frequency string `json:"frequency" binding:"required,oneof=monthly yearly"`
}

type SubscriptionResponse struct {
	Status string `json:"status"`
	Data   struct {
		User struct {
			SafeUser
			OwnerProfile service.UserProfile `json:"owner_profile"`
		} `json:"user"`
	} `json:"data"`
}

// Subscribe godoc
// @Summary      Subscribe to a plan
// @Description  Subscribe user to Discovery, Serenity, or Premium plan
// @Tags         subscriptions
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body map[string]interface{} true "Subscription Info"
// @Success      200  {object}  SubscriptionResponse
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
		logger.FromContext(c.Request.Context()).Warn("invalid subscription request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.svc.SubscribeUser(c.Request.Context(), userID, req.Plan, req.Frequency)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Fetch updated user profile
	user, err := h.userSvc.GetUserByID(c.Request.Context(), userID)
	if err != nil {
		logger.FromContext(c.Request.Context()).Error("failed to fetch user after subscription", zap.Error(err))
		// Fail gracefully, transaction succeeded but we can't show profile
		c.JSON(http.StatusCreated, gin.H{"status": "success", "message": "Subscription created, but failed to fetch profile"})
		return
	}

	authResp, err := h.userSvc.GetFullAuthResponse(c.Request.Context(), user)
	if err != nil {
		logger.FromContext(c.Request.Context()).Error("failed to construct auth response", zap.Error(err))
		c.JSON(http.StatusCreated, gin.H{"status": "success", "message": "Subscription created"})
		return
	}

	// Construct SafeUser (Reuse logic from UserHandler or duplicate if simple)
	// Duplicate for now to keep handlers decoupled, or we could share DTOs.
	// Since handler/user.go has `SafeUser` private/local, we should probably redefine or shared.
	// We'll define a quick structure here to match requirement.

	safeUser := SafeUser{
		ID:         authResp.User.ID,
		Email:      authResp.User.Email,
		FirstName:  authResp.User.FirstName.String,
		LastName:   authResp.User.LastName.String,
		Phone:      authResp.User.PhoneNumber.String,
		IsVerified: authResp.User.IsVerified.Bool,
	}
	if authResp.User.StripeCustomerID.Valid {
		safeUser.StripeCusID = authResp.User.StripeCustomerID.String
	}

	response := SubscriptionResponse{
		Status: "success",
	}
	response.Data.User.SafeUser = safeUser
	response.Data.User.OwnerProfile = authResp.Profile

	response.Data.User.OwnerProfile = authResp.Profile

	c.JSON(http.StatusOK, response)
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
		logger.FromContext(c.Request.Context()).Warn("invalid increase limit request", zap.Error(err))
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
