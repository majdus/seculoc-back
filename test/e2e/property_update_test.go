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

func TestE2E_PropertyUpdate(t *testing.T) {
	// router is initialized in TestMain

	email := fmt.Sprintf("update_owner_%d@test.com", time.Now().UnixNano())

	// 1. Register Owner
	performRequest(router, "POST", "/api/v1/auth/register", "", map[string]string{
		"email": email, "password": "password123", "first_name": "Update", "last_name": "Owner", "phone": "123",
	})

	// 2. Login Owner
	w := performRequest(router, "POST", "/api/v1/auth/login", "", map[string]string{
		"email": email, "password": "password123",
	})
	require.Equal(t, http.StatusOK, w.Code)
	var loginResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &loginResp)
	token := loginResp["token"].(string)

	// Create Subscription
	performRequest(router, "POST", "/api/v1/subscriptions", token, map[string]string{
		"plan": "discovery", "frequency": "monthly",
	})

	// 3. Create Property
	createPayload := map[string]interface{}{
		"name":           "Original Name",
		"address":        "123 Original St",
		"rental_type":    "long_term",
		"rent_amount":    1000.0,
		"deposit_amount": 2000.0,
		"details":        map[string]interface{}{"rooms": 3},
	}
	w = performRequest(router, "POST", "/api/v1/properties", token, createPayload)
	require.Equal(t, http.StatusCreated, w.Code)

	var propResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &propResp)
	propID := int(propResp["id"].(float64))

	// 4. Update Property (Success)
	updatePayload := map[string]interface{}{
		"name":                "Updated Name",
		"rent_amount":         1500.0,
		"rent_charges_amount": 150.0,
		"is_furnished":        true,
	}
	url := fmt.Sprintf("/api/v1/properties/%d", propID)
	w = performRequest(router, "PUT", url, token, updatePayload)
	require.Equal(t, http.StatusOK, w.Code)

	var updatedResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &updatedResp)

	// Assertions
	assert.Equal(t, "Updated Name", updatedResp["name"])
	assert.Equal(t, 1500.0, updatedResp["rent_amount"])
	assert.Equal(t, 150.0, updatedResp["rent_charges_amount"])
	assert.Equal(t, true, updatedResp["is_furnished"])
	assert.Equal(t, "123 Original St", updatedResp["address"]) // Should remain unchanged
	assert.Equal(t, 2000.0, updatedResp["deposit_amount"])     // Should remain unchanged

	// 5. Verify Persistence via Get
	w = performRequest(router, "GET", "/api/v1/properties", token, nil)
	require.Equal(t, http.StatusOK, w.Code)
	var listResp []map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &listResp)
	assert.NotEmpty(t, listResp)
	found := false
	for _, p := range listResp {
		if int(p["id"].(float64)) == propID {
			assert.Equal(t, "Updated Name", p["name"])
			assert.Equal(t, 150.0, p["rent_charges_amount"])
			assert.Equal(t, true, p["is_furnished"])
			found = true
			break
		}
	}
	assert.True(t, found)

	// 6. Update Forbidden (Other User)
	otherEmail := fmt.Sprintf("hacker_%d@test.com", time.Now().UnixNano())
	// Register Hacker
	performRequest(router, "POST", "/api/v1/auth/register", "", map[string]string{
		"email": otherEmail, "password": "password123", "first_name": "Hacker", "last_name": "Man", "phone": "666",
	})
	// Login Hacker
	w = performRequest(router, "POST", "/api/v1/auth/login", "", map[string]string{
		"email": otherEmail, "password": "password123",
	})
	json.Unmarshal(w.Body.Bytes(), &loginResp)
	otherToken := loginResp["token"].(string)

	w = performRequest(router, "PUT", url, otherToken, updatePayload)
	assert.Equal(t, http.StatusForbidden, w.Code)
}
