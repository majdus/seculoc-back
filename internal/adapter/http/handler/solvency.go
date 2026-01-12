package handler

import (
	"net/http"

	"seculoc-back/internal/adapter/http/middleware"
	"seculoc-back/internal/core/service"

	"github.com/gin-gonic/gin"
)

type SolvencyHandler struct {
	svc *service.SolvencyService
}

func NewSolvencyHandler(svc *service.SolvencyService) *SolvencyHandler {
	return &SolvencyHandler{svc: svc}
}

type CreateCheckRequest struct {
	CandidateEmail string `json:"candidate_email" binding:"required,email"`
	PropertyID     int32  `json:"property_id" binding:"required"`
}

func (h *SolvencyHandler) CreateCheck(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req CreateCheckRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	check, err := h.svc.RetrieveCheck(c.Request.Context(), userID, req.CandidateEmail, req.PropertyID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"check_id": check.ID, "status": check.Status})
}
