package e2e

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestE2E_InvitationFlow(t *testing.T) {
	ownerEmail := getEmail()
	tenantEmail := getEmail()

	// 1. Register Owner
	w := performRequest(router, "POST", "/api/v1/auth/register", "", map[string]string{
		"email": ownerEmail, "password": "password123", "first_name": "Owner", "last_name": "User", "phone": "123",
	})
	require.Equal(t, http.StatusCreated, w.Code)

	// 2. Login Owner
	w = performRequest(router, "POST", "/api/v1/auth/login", "", map[string]string{
		"email": ownerEmail, "password": "password123",
	})
	require.Equal(t, http.StatusOK, w.Code)

	var ownerLoginResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &ownerLoginResp)
	ownerToken, _ := ownerLoginResp["token"].(string)
	require.NotEmpty(t, ownerToken, "Owner token should not be empty")

	// Verify Owner Capabilities (Should be true now for everyone)
	caps := ownerLoginResp["capabilities"].(map[string]interface{})
	assert.True(t, caps["can_act_as_owner"].(bool))

	// 3. Owner Subscribes & Creates Property
	// Subscribe
	performRequest(router, "POST", "/api/v1/subscriptions", ownerToken, map[string]string{
		"plan": "discovery", "frequency": "monthly",
	})
	// Refresh capabilities
	w = performRequest(router, "POST", "/api/v1/auth/login", "", map[string]string{
		"email": ownerEmail, "password": "password123",
	})
	json.Unmarshal(w.Body.Bytes(), &ownerLoginResp)
	ownerToken, _ = ownerLoginResp["token"].(string)

	// Create Property with Rent/Deposit
	w = performRequest(router, "POST", "/api/v1/properties", ownerToken, map[string]interface{}{
		"address": "Invited Property", "rental_type": "long_term", "details": map[string]string{},
		"rent_amount": 1200.0, "deposit_amount": 2400.0,
	})
	require.Equal(t, http.StatusCreated, w.Code)

	var propResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &propResp)
	propID := int(propResp["id"].(float64))

	// Re-Login to refresh context/capabilities (Owner capability)
	w = performRequest(router, "POST", "/api/v1/auth/login", "", map[string]string{
		"email": ownerEmail, "password": "password123",
	})
	json.Unmarshal(w.Body.Bytes(), &ownerLoginResp)
	caps = ownerLoginResp["capabilities"].(map[string]interface{})
	assert.True(t, caps["can_act_as_owner"].(bool))

	// 4. Invite Tenant
	w = performRequest(router, "POST", "/api/v1/invitations", ownerToken, map[string]interface{}{
		"property_id": propID, "email": tenantEmail,
	})
	require.Equal(t, http.StatusCreated, w.Code)

	// In real flow, token is in email. Here (dev mode), we might return it or use backend spy?
	// The modified handler returns it in JSON for now (see step 39, line 85).
	// "message": "invitation sent", "token": inv.Token
	var invResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &invResp)
	tokenStr, _ := invResp["token"].(string)
	require.NotEmpty(t, tokenStr)

	// 5. Tenant Views Invitation (Landing Page)
	w = performRequest(router, "GET", "/api/v1/invitations/"+tokenStr, "", nil)
	require.Equal(t, http.StatusOK, w.Code)
	var invDetails map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &invDetails)
	assert.Equal(t, "Invited Property", invDetails["property_address"])
	assert.Equal(t, 1200.0, invDetails["rent_amount"])

	// 6. Tenant Register (Auto-Accepts)
	// Sticky Tenant Case
	tenantEmail = "tenant_" + randomString() + "@example.com"
	w = performRequest(router, "POST", "/api/v1/auth/register", "", map[string]interface{}{
		"email": tenantEmail, "password": "password123", "first_name": "Tenant", "last_name": "One", "phone": "0987654321",
		"invite_token": tokenStr,
	})
	require.Equal(t, http.StatusCreated, w.Code)

	// Login Tenant to Check Context
	w = performRequest(router, "POST", "/api/v1/auth/login", "", map[string]string{
		"email": tenantEmail, "password": "password123",
	})
	require.Equal(t, http.StatusOK, w.Code)

	var tenantLoginResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &tenantLoginResp)
	// tenantToken is not used, but we keep the structure clean
	var tenant, _ = tenantLoginResp["token"].(string)
	_ = tenant

	// 7. Verification: Check Tenant Context/Capabilities
	// No need to explicitly accept anymore.

	// 7. Verification: Check Tenant Context/Capabilities
	// Re-Login Tenant
	w = performRequest(router, "POST", "/api/v1/auth/login", "", map[string]string{
		"email": tenantEmail, "password": "password123",
	})
	require.Equal(t, http.StatusOK, w.Code)

	json.Unmarshal(w.Body.Bytes(), &tenantLoginResp)
	tenantCaps := tenantLoginResp["capabilities"].(map[string]interface{})

	assert.True(t, tenantCaps["can_act_as_tenant"].(bool))
	// Context might default to Tenant if only Tenant?
	// or remain Owner/None until switched.
	// My logic: if Owner -> 'owner', else if Tenant -> 'tenant'.
	// This tenant has no properties, BUT 'can_act_as_owner' is now true for everyone (Airbnb style).
	assert.True(t, tenantCaps["can_act_as_owner"].(bool))
	assert.Equal(t, "tenant", tenantLoginResp["current_context"])
}
