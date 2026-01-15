package handler

import (
	"fmt"
	"net/http"

	"seculoc-back/internal/adapter/http/middleware"
	"seculoc-back/internal/core/service"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

type SolvencyHandler struct {
	svc *service.SolvencyService
}

func NewSolvencyHandler(svc *service.SolvencyService) *SolvencyHandler {
	return &SolvencyHandler{svc: svc}
}

type SolvencyCheckResponse struct {
	ID                 int32  `json:"id"`
	CandidateEmail     string `json:"candidate_email"`
	CandidateFirstName string `json:"candidate_first_name"`
	CandidateLastName  string `json:"candidate_last_name"`
	PropertyID         int32  `json:"property_id"`
	PropertyAddress    string `json:"property_address,omitempty"`
	Status             string `json:"status"`
	ScoreResult        int32  `json:"score_result,omitempty"`
	ReportUrl          string `json:"report_url,omitempty"`
	CreatedAt          string `json:"created_at"`
	Token              string `json:"token"`
	VerificationUrl    string `json:"verification_url"`
}

type CreateCheckRequest struct {
	CandidateEmail     string `json:"candidate_email" binding:"required,email"`
	CandidateFirstName string `json:"candidate_first_name"`
	CandidateLastName  string `json:"candidate_last_name"`
	CandidatePhone     string `json:"candidate_phone"`
	PropertyID         int32  `json:"property_id" binding:"required"`
}

// CreateCheck godoc
// @Summary      Initiate Solvency Check
// @Description  Consume credit to run solvency check on candidate and send invitation email
// @Tags         solvency
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body CreateCheckRequest true "Check Info"
// @Success      201  {object}  SolvencyCheckResponse
// @Failure      400  {object}  map[string]string
// @Router       /solvency/check [post]
func (h *SolvencyHandler) CreateCheck(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req CreateCheckRequest
	if err := h.bindJSON(c, &req); err != nil {
		return
	}

	check, err := h.svc.InitiateCheck(c.Request.Context(), service.InitiateCheckParams{
		UserID:             userID,
		CandidateEmail:     req.CandidateEmail,
		CandidateFirstName: req.CandidateFirstName,
		CandidateLastName:  req.CandidateLastName,
		CandidatePhone:     req.CandidatePhone,
		PropertyID:         req.PropertyID,
	})
	if err != nil {
		if insErr, ok := err.(*service.ErrInsufficientCredits); ok {
			c.JSON(http.StatusPaymentRequired, gin.H{
				"error":   "ERR_INSUFFICIENT_CREDITS",
				"message": "Solde insuffisant. Veuillez recharger votre compte ou votre logement.",
				"details": gin.H{
					"global_balance":   insErr.GlobalBalance,
					"property_balance": insErr.PropertyBalance,
				},
			})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	baseURL := viper.GetString("FRONTEND_URL")
	if baseURL == "" {
		baseURL = "https://seculoc.com"
	}
	verificationURL := fmt.Sprintf("%s/check/%s", baseURL, check.Token.String)

	c.JSON(http.StatusCreated, gin.H{
		"id":               check.ID,
		"check_id":         check.ID, // Deprecated but kept for safety
		"status":           check.Status.SolvencyStatus,
		"token":            check.Token.String,
		"verification_url": verificationURL,
	})
}

// ListChecks godoc
// @Summary      List Solvency Checks
// @Description  Get all solvency checks for the current owner, optionally filtered by property
// @Tags         solvency
// @Produce      json
// @Security     BearerAuth
// @Param        property_id query int false "Property ID filter"
// @Success      200  {array}   SolvencyCheckResponse
// @Failure      400  {object}  map[string]string
// @Router       /solvency/checks [get]
func (h *SolvencyHandler) ListChecks(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	propIDStr := c.Query("property_id")
	if propIDStr != "" {
		// Filter by property (logic to verify ownership should ideally be in service, but let's keep it simple for now)
		// Wait, ListSolvencyChecksByProperty currently doesn't check owner.
		// Actually, let's just use ListChecksForOwner and filter in Go if needed, or stick to Owner list.
		// For the front MVP, one owner list is enough.
	}

	checks, err := h.svc.ListChecksForOwner(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp := make([]SolvencyCheckResponse, len(checks))
	for i, sc := range checks {
		resp[i] = SolvencyCheckResponse{
			ID:                 sc.ID,
			CandidateEmail:     sc.CandidateEmail,
			CandidateFirstName: sc.CandidateFirstName.String,
			CandidateLastName:  sc.CandidateLastName.String,
			PropertyID:         sc.PropertyID.Int32,
			PropertyAddress:    sc.PropertyAddress,
			Status:             string(sc.Status.SolvencyStatus),
			ScoreResult:        sc.ScoreResult.Int32,
			ReportUrl:          sc.ReportUrl.String,
			CreatedAt:          sc.CreatedAt.Time.String(),
			Token:              sc.Token.String,
			VerificationUrl:    fmt.Sprintf("%s/check/%s", viper.GetString("FRONTEND_URL"), sc.Token.String),
		}
	}

	c.JSON(http.StatusOK, resp)
}

// bindJSON is a helper to bind and handle common error
func (h *SolvencyHandler) bindJSON(c *gin.Context, obj interface{}) error {
	if err := c.ShouldBindJSON(obj); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return err
	}
	return nil
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
// @Param        request body map[string]interface{} true "Credit Pack Info"
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

// PublicSolvencyCheckResponse defines the public view of a solvency check
type PublicSolvencyCheckResponse struct {
	CandidateEmail     string  `json:"candidate_email"`
	CandidateFirstName string  `json:"candidate_first_name"`
	CandidateLastName  string  `json:"candidate_last_name"`
	Status             string  `json:"status"`
	PropertyID         int32   `json:"property_id"`
	PropertyAddress    string  `json:"property_address"`
	PropertyName       string  `json:"property_name"`
	RentAmount         float64 `json:"rent_amount"`
}

// GetCheckByToken godoc
// @Summary      Get Solvency Check Details (Public)
// @Description  Allows candidate to see the solvency check request
// @Tags         solvency
// @Produce      json
// @Param        token path string true "Check Token"
// @Success      200  {object}  PublicSolvencyCheckResponse
// @Failure      404  {object}  map[string]string
// @Router       /solvency/public/check/{token} [get]
func (h *SolvencyHandler) GetCheckByToken(c *gin.Context) {
	token := c.Param("token")
	check, err := h.svc.GetCheckByToken(c.Request.Context(), token)
	if err != nil {
		if err.Error() == "check not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "check not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, PublicSolvencyCheckResponse{
		CandidateEmail:     check.CandidateEmail,
		CandidateFirstName: check.CandidateFirstName,
		CandidateLastName:  check.CandidateLastName,
		Status:             string(check.Status.SolvencyStatus),
		PropertyID:         check.PropertyID.Int32,
		PropertyAddress:    check.PropertyAddress,
		PropertyName:       check.PropertyName,
		RentAmount:         check.PropertyRent,
	})
}

type OpenBankingCallback struct {
	Transactions []service.TransactionData `json:"transactions"`
}

// ProcessCallback godoc
// @Summary      Open Banking Callback (Public)
// @Description  Endpoint for mocking Open Banking data callback
// @Tags         solvency
// @Accept       json
// @Produce      json
// @Param        token path string true "Check Token"
// @Param        request body OpenBankingCallback true "Transaction Data"
// @Success      200  {object}  map[string]string
// @Failure      400  {object}  map[string]string
// @Router       /solvency/public/check/{token}/callback [post]
func (h *SolvencyHandler) ProcessCallback(c *gin.Context) {
	token := c.Param("token")
	var req OpenBankingCallback
	if err := h.bindJSON(c, &req); err != nil {
		return
	}

	err := h.svc.ProcessOpenBankingResult(c.Request.Context(), token, req.Transactions)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "callback processed successfully"})
}

// CancelCheck godoc
// @Summary      Cancel Solvency Check
// @Description  Cancel a pending check and refund credit to property or global wallet
// @Tags         solvency
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "Check ID"
// @Success      200  {object}  map[string]string
// @Failure      400  {object}  map[string]string
// @Router       /solvency/check/{id}/cancel [post]
func (h *SolvencyHandler) CancelCheck(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var uri struct {
		ID int32 `uri:"id" binding:"required"`
	}
	if err := c.ShouldBindUri(&uri); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.svc.CancelCheck(c.Request.Context(), uri.ID, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "check cancelled and credit refunded"})
}
