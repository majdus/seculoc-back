package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestE2E_DeleteProperty(t *testing.T) {
	email := getEmail()

	// 1. Register
	w := performRequest(router, "POST", "/api/v1/auth/register", "", map[string]string{
		"email": email, "password": "password123", "first_name": "Del", "last_name": "User", "phone_number": "999",
	})
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

	// 3. Subscribe (required for limits, though seasonal is open)
	w = performRequest(router, "POST", "/api/v1/subscriptions", token, map[string]string{
		"plan": "discovery", "frequency": "monthly",
	})
	require.Equal(t, http.StatusCreated, w.Code)

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
