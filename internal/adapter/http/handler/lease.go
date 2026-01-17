package handler

import (
	"fmt"
	"net/http"
	"strconv"

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

// Preview godoc
// @Summary      Preview lease document
// @Description  Get the lease contract as HTML for display
// @Tags         leases
// @Produce      html
// @Security     BearerAuth
// @Param        id   path      int  true  "Lease ID"
// @Success      200  {string}  string "HTML Content"
// @Failure      400  {object}  map[string]string
// @Failure      403  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /leases/{id}/preview [get]
func (h *LeaseHandler) Preview(c *gin.Context) {
	// 1. Get UserID
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid lease id"})
		return
	}

	// Use GetLeaseDocumentContent (HTML)
	content, _, err := h.svc.GetLeaseDocumentContent(c.Request.Context(), int32(id), userID)
	if err != nil {
		if err.Error() == fmt.Sprintf("access denied: user %d is not a party to this lease", userID) {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Return inline HTML
	c.Data(http.StatusOK, "text/html; charset=utf-8", content)
}

// Download godoc
// @Summary      Download lease document (PDF)
// @Description  Generate and download the lease contract as PDF
// @Tags         leases
// @Produce      application/pdf
// @Security     BearerAuth
// @Param        id   path      int  true  "Lease ID"
// @Success      200  {file}    file
// @Failure      400  {object}  map[string]string
// @Failure      403  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /leases/{id}/download [get]
func (h *LeaseHandler) Download(c *gin.Context) {
	// 1. Get UserID
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid lease id"})
		return
	}

	// Generate PDF
	pdfBytes, filename, err := h.svc.GenerateLeasePDF(c.Request.Context(), int32(id), userID)
	if err != nil {
		if err.Error() == fmt.Sprintf("access denied: user %d is not a party to this lease", userID) {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	c.Data(http.StatusOK, "application/pdf", pdfBytes)
}

// CreateDraft godoc
// @Summary      Create a draft lease
// @Description  Create a new lease in draft mode and invite the tenant
// @Tags         leases
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body service.DraftLeaseRequest true "Draft Lease Details"
// @Success      201  {object}  map[string]interface{}
// @Failure      400  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /leases/draft [post]
func (h *LeaseHandler) CreateDraft(c *gin.Context) {
	// 1. Get Owner ID
	ownerID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	// 2. Bind JSON
	var req service.DraftLeaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 3. Call Service
	leaseID, token, err := h.svc.CreateDraft(c.Request.Context(), req, ownerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 4. Return Success
	c.JSON(http.StatusCreated, gin.H{
		"id":      leaseID,
		"token":   token,
		"status":  "draft",
		"message": "Lease created and invitation sent.",
	})
}
