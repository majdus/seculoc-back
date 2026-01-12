package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestE2E_DeleteProperty(t *testing.T) {
	// 1. Register User
	email := "e2e_" + fmt.Sprintf("%d", time.Now().UnixNano()) + "@example.com"
	payload := map[string]interface{}{
		"email":      email,
		"password":   "password123",
		"first_name": "John",
		"last_name":  "Doe",
		"phone":      "1234567890",
	}
	w := performRequest(router, "POST", "/api/v1/auth/register", "", payload)
	require.Equal(t, http.StatusCreated, w.Code)

	// 2. Login
	w = performRequest(router, "POST", "/api/v1/auth/login", "", map[string]string{
		"email": email, "password": "password123",
	})
	require.Equal(t, http.StatusOK, w.Code)

	var loginResp map[string]string
	json.Unmarshal(w.Body.Bytes(), &loginResp)
	token := loginResp["token"]
	require.NotEmpty(t, token)

	// 3. Subscribe (required to create property)
	w = performRequest(router, "POST", "/api/v1/subscriptions", token, map[string]string{
		"plan": "discovery", "frequency": "monthly",
	})
	require.Equal(t, http.StatusOK, w.Code)

	// 4. Create Property
	w = performRequest(router, "POST", "/api/v1/properties", token, map[string]interface{}{
		"address": "To Delete St.", "rental_type": "seasonal", "details": map[string]string{},
	})
	require.Equal(t, http.StatusCreated, w.Code)

	var propResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &propResp)
	var propID int
	if pid, ok := propResp["property_id"].(float64); ok {
		propID = int(pid)
	}
	require.NotZero(t, propID)

	// 5. Verify it exists in List
	w = performRequest(router, "GET", "/api/v1/properties", token, nil)
	require.Equal(t, http.StatusOK, w.Code)
	// We expect 1 property
	var listResp []map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &listResp)
	require.Len(t, listResp, 1)

	// 6. Delete Property
	deleteURL := fmt.Sprintf("/api/v1/properties/%d", propID)
	w = performRequest(router, "DELETE", deleteURL, token, nil)
	require.Equal(t, http.StatusNoContent, w.Code)

	// 7. Verify it is gone from List (Soft Deleted)
	w = performRequest(router, "GET", "/api/v1/properties", token, nil)
	require.Equal(t, http.StatusOK, w.Code)

	var listRespAfter []map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &listRespAfter)
	require.Len(t, listRespAfter, 0)

	// 8. Try to Delete again (Should be 403 Forbidden - Not Found/Access Denied)
	w = performRequest(router, "DELETE", deleteURL, token, nil)
	require.Equal(t, http.StatusForbidden, w.Code)
}
