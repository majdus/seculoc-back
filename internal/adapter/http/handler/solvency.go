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

// CreateCheck godoc
// @Summary      Initiate Solvency Check
// @Description  Consume credit to run solvency check on candidate
// @Tags         solvency
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body CheckRequest true "Check Info"
// @Success      201  {object}  map[string]interface{}
// @Failure      400  {object}  map[string]string
// @Router       /solvency/check [post]
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

type BuyCreditsRequest struct {
	PackType string `json:"pack_type" binding:"required,oneof=pack_20"`
}

// BuyCredits godoc
// @Summary      Purchase Credit Pack
// @Description  Buy solvency check credits (e.g., pack_20)
// @Tags         solvency
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body CreditRequest true "Credit Pack Info"
// @Success      200  {object}  map[string]interface{}
// @Failure      400  {object}  map[string]string
// @Router       /solvency/credits [post]
func (h *SolvencyHandler) BuyCredits(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req BuyCreditsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	amount, err := h.svc.PurchaseCredits(c.Request.Context(), userID, req.PackType)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "credits purchased", "added": amount})
}
