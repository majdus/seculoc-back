package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

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

// PropertyResponse represents the property object returned in API
type PropertyResponse struct {
	ID             int32                  `json:"id"`
	OwnerID        int32                  `json:"owner_id"`
	Address        string                 `json:"address"`
	RentalType     string                 `json:"rental_type"`
	Details        map[string]interface{} `json:"details"`
	VacancyCredits int32                  `json:"vacancy_credits"`
	IsActive       bool                   `json:"is_active"`
	CreatedAt      string                 `json:"created_at"`
}

// Create godoc
// @Summary      Create a new property
// @Description  Create a property listing (Long Term or Seasonal)
// @Tags         properties
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body map[string]interface{} true "Property Info"
// @Success      201  {object}  PropertyResponse
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

	c.JSON(http.StatusCreated, prop)
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

// Delete godoc
// @Summary      Delete a property
// @Description  Soft delete a property belonging to the authenticated user
// @Tags         properties
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      int  true  "Property ID"
// @Success      204  {object}  nil
// @Failure      400  {object}  map[string]string
// @Failure      403  {object}  map[string]string
// @Router       /properties/{id} [delete]
func (h *PropertyHandler) Delete(c *gin.Context) {
	log := logger.FromContext(c.Request.Context())

	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid property id"})
		return
	}

	err = h.svc.DeleteProperty(c.Request.Context(), userID, int32(id))
	if err != nil {
		log.Warn("delete property failed", zap.Error(err))
		if err.Error() == "property not found or access denied" {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}
