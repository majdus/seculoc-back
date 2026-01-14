package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"seculoc-back/internal/core/service"
	"seculoc-back/internal/platform/auth"
	"seculoc-back/internal/platform/logger"
)

type UserHandler struct {
	svc *service.UserService
}

func NewUserHandler(svc *service.UserService) *UserHandler {
	return &UserHandler{svc: svc}
}

type RegisterRequest struct {
	Email       string `json:"email" binding:"required,email"`
	Password    string `json:"password" binding:"required,min=8"`
	FirstName   string `json:"first_name" binding:"required"`
	LastName    string `json:"last_name" binding:"required"`
	Phone       string `json:"phone" binding:"required"`
	InviteToken string `json:"invite_token"` // Optional
}

// Register godoc
// @Summary      Register a new user
// @Description  Create a new user account
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body RegisterRequest true "Registration Info"
// @Success      201  {object}  map[string]interface{}
// @Failure      400  {object}  map[string]string
// @Router       /auth/register [post]
func (h *UserHandler) Register(c *gin.Context) {
	log := logger.FromContext(c.Request.Context())
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warn("invalid register request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := h.svc.Register(c.Request.Context(), req.Email, req.Password, req.FirstName, req.LastName, req.Phone, req.InviteToken)
	if err != nil {
		// Distinguish errors (conflict vs internal)
		// For simplicity, returning 500 or 400 based on message usually,
		// but better to have typed errors.
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"user_id": user.ID, "email": user.Email})
}

// SafeUser defines the public user fields
type SafeUser struct {
	ID          int32  `json:"id"`
	Email       string `json:"email"`
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	Phone       string `json:"phone"`
	IsVerified  bool   `json:"is_verified"`
	StripeCusID string `json:"stripe_customer_id,omitempty"`
}

// LoginResponse defines the structure of the login response
type LoginResponse struct {
	Token          string               `json:"token"`
	CurrentContext service.UserContext  `json:"current_context"`
	Capabilities   service.Capabilities `json:"capabilities"`
	User           struct {
		SafeUser
		OwnerProfile service.UserProfile `json:"owner_profile"`
	} `json:"user"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// Login godoc
// @Summary      Login user
// @Description  Authenticate user and return JWT token
// @Tags         auth
// @Accept       json
// @Accept       json
// @Produce      json
// @Param        request body LoginRequest true "Login Credentials"
// @Success      200  {object}  LoginResponse
// @Failure      400  {object}  map[string]string
// @Failure      401  {object}  map[string]string
// @Router       /auth/login [post]
func (h *UserHandler) Login(c *gin.Context) {
	log := logger.FromContext(c.Request.Context())
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	authResp, err := h.svc.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		// Invalid credentials usually
		log.Warn("login failed", zap.String("email", req.Email), zap.Error(err))
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	// Generate Token
	token, err := auth.GenerateToken(authResp.User.ID, authResp.User.Email, string(authResp.CurrentContext))
	if err != nil {
		log.Error("failed to generate token", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	// Construct SafeUser
	safeUser := SafeUser{
		ID:         authResp.User.ID,
		Email:      authResp.User.Email,
		FirstName:  authResp.User.FirstName.String,
		LastName:   authResp.User.LastName.String,
		Phone:      authResp.User.PhoneNumber.String,
		IsVerified: authResp.User.IsVerified.Bool,
	}
	if authResp.User.StripeCustomerID.Valid {
		safeUser.StripeCusID = authResp.User.StripeCustomerID.String
	}

	response := LoginResponse{
		Token:          token,
		CurrentContext: authResp.CurrentContext,
		Capabilities:   authResp.Capabilities,
	}
	response.User.SafeUser = safeUser
	response.User.OwnerProfile = authResp.Profile

	c.JSON(http.StatusOK, response)
}

type SwitchContextRequest struct {
	TargetContext string `json:"target_context" binding:"required,oneof=owner tenant"`
}

// SwitchContext godoc
// @Summary      Switch user context
// @Description  Switch between owner and tenant context
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body SwitchContextRequest true "Context Info"
// @Success      200  {object}  LoginResponse
// @Failure      400  {object}  map[string]string
// @Router       /auth/switch-context [post]
func (h *UserHandler) SwitchContext(c *gin.Context) {
	log := logger.FromContext(c.Request.Context())

	// Get User ID from context
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req SwitchContextRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Call service to switch context and get fresh auth data
	authResp, err := h.svc.SwitchContext(c.Request.Context(), userID.(int32), req.TargetContext)
	if err != nil {
		log.Error("failed to switch context", zap.Error(err))
		// Check for specific error messages (e.g. capability) could improve UX
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Generate NEW Token with updated context claims if needed
	// Assuming GenerateToken encodes basic info. If it encodes context, we need to pass it.
	// Current GenerateToken(id, email) -> It likely does NOT encode context?
	// Wait, if it doesn't encode context, why did the user say "if the token doesn't change, the frontend remains blocked"?
	// Ah, maybe the frontend relies on claims in the token for "context".
	// Let's verify auth.GenerateToken. Even if it doesn't, returning a new token acts as a signal.
	// But critically: The User Request said "GÉNÉRER UN NOUVEAU TOKEN JWT... Ce token doit contenir le claim current_context".

	// I need to check auth.GenerateToken signature.
	// Assuming for now I can call it.
	token, err := auth.GenerateToken(authResp.User.ID, authResp.User.Email, string(authResp.CurrentContext))
	if err != nil {
		log.Error("failed to generate token", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	// Prepare Response (Same as LoginResponse)
	// Construct SafeUser
	safeUser := SafeUser{
		ID:         authResp.User.ID,
		Email:      authResp.User.Email,
		FirstName:  authResp.User.FirstName.String,
		LastName:   authResp.User.LastName.String,
		Phone:      authResp.User.PhoneNumber.String,
		IsVerified: authResp.User.IsVerified.Bool,
	}
	if authResp.User.StripeCustomerID.Valid {
		safeUser.StripeCusID = authResp.User.StripeCustomerID.String
	}

	response := LoginResponse{
		Token:          token,
		CurrentContext: authResp.CurrentContext,
		Capabilities:   authResp.Capabilities,
	}
	response.User.SafeUser = safeUser
	response.User.OwnerProfile = authResp.Profile

	c.JSON(http.StatusOK, response)
}
