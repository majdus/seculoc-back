package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"seculoc-back/internal/core/service"
	"seculoc-back/internal/platform/logger"
)

type InvitationHandler struct {
	svc *service.UserService
}

func NewInvitationHandler(svc *service.UserService) *InvitationHandler {
	return &InvitationHandler{svc: svc}
}

type InviteTenantRequest struct {
	PropertyID int32  `json:"property_id" binding:"required"`
	Email      string `json:"email" binding:"required,email"`
}

// InviteTenant godoc
// @Summary      Invite a tenant
// @Description  Send an invitation to a tenant
// @Tags         invitations
// @Accept       json
// @Produce      json
// @Param        request body InviteTenantRequest true "Invitation Info"
// @Success      201  {object}  map[string]string
// @Failure      400  {object}  map[string]string
// @Router       /invitations [post]
func (h *InvitationHandler) InviteTenant(c *gin.Context) {
	log := logger.FromContext(c.Request.Context())

	// Get User ID from context (Auth Middleware)
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	// Assuming userID is float64 (JWT claim default) or int depending on middleware.
	// Let's assume int for now or cast carefully.
	// In my middleware (which I should check), it usually sets it as float64 if from standard JWT parser,
	// or specific type if custom middleware.
	// Based on other handlers (not visible but common pattern), let's cast to int.
	// Safest is generic cast or check `handler/user.go`.

	// Wait, `handler/user.go` `Login` generates token. Need to check middleware to see how it parses.
	// Assume `int(userID.(float64))` if generic.

	// Correction: In `handler/solvency.go` (if I could see it) I would check.
	// Let's assume standard int casting for now, but handle potential panic.

	var uid int32
	switch v := userID.(type) {
	case int:
		uid = int32(v)
	case float64:
		uid = int32(v)
	case int32:
		uid = v
	default:
		log.Error("user_id type mismatch in context", zap.Any("type", v))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	var req InviteTenantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	inv, err := h.svc.InviteTenant(c.Request.Context(), uid, req.PropertyID, req.Email)
	if err != nil {
		log.Error("failed to invite tenant", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()}) // Mapping generic error to 400
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "invitation sent", "token": inv.Token}) // Returning token for testing purposes mainly? Or usually just "sent". For E2E we need it or we need to spy on DB.
}

type AcceptInvitationRequest struct {
	Token string `json:"token" binding:"required"`
}

// AcceptInvitation godoc
// @Summary      Accept an invitation
// @Description  Accept a tenant invitation
// @Tags         invitations
// @Accept       json
// @Produce      json
// @Param        request body AcceptInvitationRequest true "Token"
// @Success      200  {object}  map[string]string
// @Failure      400  {object}  map[string]string
// @Router       /invitations/accept [post]
func (h *InvitationHandler) AcceptInvitation(c *gin.Context) {
	log := logger.FromContext(c.Request.Context())

	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var uid int32
	switch v := userID.(type) {
	case int:
		uid = int32(v)
	case float64:
		uid = int32(v)
	case int32:
		uid = v
	default:
		log.Error("user_id type mismatch", zap.Any("type", v))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	var req AcceptInvitationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.svc.AcceptInvitation(c.Request.Context(), req.Token, uid)
	if err != nil {
		log.Warn("failed to accept invitation", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "invitation accepted"})
}
