package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestE2E_InvitationEdgeCases(t *testing.T) {
	// 1. Invalid Token
	t.Run("Invalid Token", func(t *testing.T) {
		w := performRequest(router, "GET", "/api/v1/invitations/invalid-token-123", "", nil)
		require.Equal(t, http.StatusBadRequest, w.Code) // Controller return 400 with error

		// Attempt Register
		email := "invalid_token_" + fmt.Sprintf("%d", time.Now().UnixNano()) + "@example.com"
		w = performRequest(router, "POST", "/api/v1/auth/register", "", map[string]interface{}{
			"email": email, "password": "password", "first_name": "Test", "last_name": "Invalid", "phone": "123",
			"invite_token": "invalid-token-123",
		})
		// It should fail or ignore token?
		// Current logic: If token invalid, Register returns error "invalid invitation token"
		require.NotEqual(t, http.StatusCreated, w.Code)
		var resp map[string]string
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Contains(t, resp["error"], "invitation")
	})

	// 2. Expired Token (Difficult to test E2E without hacking DB or mock time, skipping for now unless we insert raw logic)

}
