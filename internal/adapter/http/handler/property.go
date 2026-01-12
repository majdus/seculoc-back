package handler

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"seculoc-back/internal/adapter/http/middleware"
	"seculoc-back/internal/core/service"
	"seculoc-back/internal/platform/logger"
)

type PropertyHandler struct {
	svc *service.PropertyService
}

func NewPropertyHandler(svc *service.PropertyService) *PropertyHandler {
	return &PropertyHandler{svc: svc}
}

type CreatePropertyRequest struct {
	Address    string                 `json:"address" binding:"required"`
	RentalType string                 `json:"rental_type" binding:"required,oneof=long_term seasonal"`
	Details    map[string]interface{} `json:"details" binding:"required"`
}

// Create godoc
// @Summary      Create a new property
// @Description  Create a property listing (Long Term or Seasonal)
// @Tags         properties
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body map[string]interface{} true "Property Info"
// @Success      201  {object}  map[string]interface{}
// @Failure      400  {object}  map[string]string
// @Failure      403  {object}  map[string]string
// @Router       /properties [post]
func (h *PropertyHandler) Create(c *gin.Context) {
	log := logger.FromContext(c.Request.Context())

	// Get User ID from Auth Middleware
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req CreatePropertyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Convert details map to JSON string
	detailsBytes, err := json.Marshal(req.Details)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid details format"})
		return
	}

	prop, err := h.svc.CreateProperty(c.Request.Context(), userID, req.Address, req.RentalType, string(detailsBytes))
	if err != nil {
		log.Warn("create property failed", zap.Error(err))
		// Ideally verify if quota error or system error
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"property_id": prop.ID})
}

// List godoc
// @Summary      List user properties
// @Description  Get all properties belonging to the authenticated user
// @Tags         properties
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {array}   map[string]interface{}
// @Router       /properties [get]
func (h *PropertyHandler) List(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	props, err := h.svc.ListProperties(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, props)
}
