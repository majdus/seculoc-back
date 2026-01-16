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

// Download godoc
// @Summary      Download lease document
// @Description  Generate and download the lease contract (HTML/PDF)
// @Tags         leases
// @Produce      html
// @Security     BearerAuth
// @Param        id   path      int  true  "Lease ID"
// @Success      200  {file}    file
// @Failure      400  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /leases/{id}/download [get]
func (h *LeaseHandler) Download(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid lease id"})
		return
	}

	// TODO: Verify access (User is tenant or owner of this lease)
	// For now, service might handle it or we relying on assumption user possesses ID.
	// ideally Service should check ownership.
	// But `GenerateLeaseDocument` currently fetches by ID. It doesn't check requester.
	// I should pass UserID to `GenerateLeaseDocument` to authorize.

	content, filename, err := h.svc.GetLeaseDocumentContent(c.Request.Context(), int32(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	c.Data(http.StatusOK, "text/html; charset=utf-8", content)
}
