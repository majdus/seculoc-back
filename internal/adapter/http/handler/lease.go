package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"seculoc-back/internal/adapter/http/middleware"
	"seculoc-back/internal/core/service"
)

type LeaseHandler struct {
	svc *service.LeaseService
}

func NewLeaseHandler(svc *service.LeaseService) *LeaseHandler {
	return &LeaseHandler{svc: svc}
}

// List godoc
// @Summary      List user leases
// @Description  Get all active leases where the authenticated user is the tenant
// @Tags         leases
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {array}   service.LeaseDTO
// @Failure      401  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /leases [get]
func (h *LeaseHandler) List(c *gin.Context) {
	// 1. Get User ID from Token (Implicit Isolation)
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	// 2. Call Service
	leases, err := h.svc.ListLeases(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 3. Return Response (Empty list handled by service)
	c.JSON(http.StatusOK, leases)
}
