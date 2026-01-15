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
	Name          string                 `json:"name"`
	Address       string                 `json:"address" binding:"required"`
	RentalType    string                 `json:"rental_type" binding:"required,oneof=long_term seasonal"`
	Details       map[string]interface{} `json:"details" binding:"required"`
	RentAmount    float64                `json:"rent_amount"`
	DepositAmount float64                `json:"deposit_amount"`
}

type UpdatePropertyRequest struct {
	Name          string                 `json:"name"`
	Address       string                 `json:"address"`
	RentalType    string                 `json:"rental_type" binding:"omitempty,oneof=long_term seasonal"`
	Details       map[string]interface{} `json:"details"`
	RentAmount    float64                `json:"rent_amount"`
	DepositAmount float64                `json:"deposit_amount"`
}

// PropertyResponse represents the property object returned in API
type PropertyResponse struct {
	ID             int32                  `json:"id"`
	OwnerID        int32                  `json:"owner_id"`
	Name           string                 `json:"name"`
	Address        string                 `json:"address"`
	RentalType     string                 `json:"rental_type"`
	Details        map[string]interface{} `json:"details"`
	RentAmount     float64                `json:"rent_amount"`
	DepositAmount  float64                `json:"deposit_amount"`
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

	detailsJSON, err := json.Marshal(req.Details)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid details json"})
		return
	}

	prop, err := h.svc.CreateProperty(c.Request.Context(), userID, req.Name, req.Address, req.RentalType, string(detailsJSON), req.RentAmount, req.DepositAmount)
	if err != nil {
		if err.Error() == "property quota exceeded for current plan" {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		log.Error("failed to create property", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var detailsMap map[string]interface{}
	json.Unmarshal(prop.Details, &detailsMap)

	deposit, _ := prop.DepositAmount.Float64Value()
	rent, _ := prop.RentAmount.Float64Value()

	c.JSON(http.StatusCreated, PropertyResponse{
		ID:             prop.ID,
		OwnerID:        prop.OwnerID.Int32,
		Name:           prop.Name.String,
		Address:        prop.Address,
		RentalType:     string(prop.RentalType),
		Details:        detailsMap,
		RentAmount:     rent.Float64,
		DepositAmount:  deposit.Float64,
		VacancyCredits: prop.VacancyCredits,
		IsActive:       prop.IsActive.Bool,
		CreatedAt:      prop.CreatedAt.Time.String(),
	})
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
// Update godoc
// @Summary      Update a property
// @Description  Update details of an existing property
// @Tags         properties
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      int  true  "Property ID"
// @Param        request body UpdatePropertyRequest true "Property Update Info"
// @Success      200  {object}  PropertyResponse
// @Failure      400  {object}  map[string]string
// @Failure      403  {object}  map[string]string
// @Router       /properties/{id} [put]
func (h *PropertyHandler) Update(c *gin.Context) {
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

	var req UpdatePropertyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var detailsJSON []byte
	if req.Details != nil {
		b, err := json.Marshal(req.Details)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid details json"})
			return
		}
		detailsJSON = b
	}

	prop, err := h.svc.UpdateProperty(c.Request.Context(), userID, int32(id), req.Name, req.Address, req.RentalType, string(detailsJSON), req.RentAmount, req.DepositAmount)
	if err != nil {
		log.Error("failed to update property", zap.Error(err))
		if err.Error() == "property not found or access denied" {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var detailsMap map[string]interface{}
	json.Unmarshal(prop.Details, &detailsMap)

	deposit, _ := prop.DepositAmount.Float64Value()
	rent, _ := prop.RentAmount.Float64Value()

	c.JSON(http.StatusOK, PropertyResponse{
		ID:             prop.ID,
		OwnerID:        prop.OwnerID.Int32,
		Name:           prop.Name.String,
		Address:        prop.Address,
		RentalType:     string(prop.RentalType),
		Details:        detailsMap,
		RentAmount:     rent.Float64,
		DepositAmount:  deposit.Float64,
		VacancyCredits: prop.VacancyCredits,
		IsActive:       prop.IsActive.Bool,
		CreatedAt:      prop.CreatedAt.Time.String(),
	})
}

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
