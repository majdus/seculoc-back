package e2e

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestE2E_LeaseWizard_ChargesPersistence(t *testing.T) {
	ownerEmail := "owner_wiz_" + randomString() + "@example.com"
	tenantEmail := "tenant_wiz_" + randomString() + "@example.com"

	// 1. Register Owner
	registerAndLogin(t, ownerEmail, "Owner", "Wizard")
	ownerToken := login(t, ownerEmail, "password123")

	// 1.5 Owner Subscribes
	w := performRequest(router, "POST", "/api/v1/subscriptions", ownerToken, map[string]string{
		"plan": "discovery", "frequency": "monthly",
	})
	require.Equal(t, http.StatusOK, w.Code)
	// Login again to refresh capabilities (standard procedure in this app)
	ownerToken = login(t, ownerEmail, "password123")

	// 2. Create Property with Default Charges
	w = performRequest(router, "POST", "/api/v1/properties", ownerToken, map[string]interface{}{
		"address": "Wizard Tower", "rental_type": "long_term", "details": map[string]string{},
		"rent_amount": 1000.0, "rent_charges_amount": 150.0, "deposit_amount": 1000.0,
	})
	require.Equal(t, http.StatusCreated, w.Code)
	var propResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &propResp)
	propID := int(propResp["id"].(float64))

	// 3. Create Draft Lease with CUSTOM Charges
	w = performRequest(router, "POST", "/api/v1/leases/draft", ownerToken, map[string]interface{}{
		"property_id": propID,
		"tenant_info": map[string]string{
			"first_name": "Jean", "last_name": "Wizard", "email": tenantEmail,
		},
		"terms": map[string]interface{}{
			"start_date":     "2026-03-01",
			"rent_amount":    900.0,
			"charges_amount": 200.0, // Different from property default (150)
			"deposit_amount": 900.0,
			"payment_day":    10,
		},
		"clauses": []string{"No magic allowed"},
	})
	require.Equal(t, http.StatusCreated, w.Code)
	var wizResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &wizResp)
	tokenStr := wizResp["token"].(string)
	leaseID := int(wizResp["id"].(float64))
	require.NotEmpty(t, tokenStr)

	// 4. Tenant Register with Token (Accepts the Draft)
	w = performRequest(router, "POST", "/api/v1/auth/register", "", map[string]interface{}{
		"email": tenantEmail, "password": "password123", "first_name": "Jean", "last_name": "Wizard", "phone": "0000",
		"invite_token": tokenStr,
	})
	require.Equal(t, http.StatusCreated, w.Code)

	tenantToken := login(t, tenantEmail, "password123")

	// 5. Verify Charges in ListLeases
	w = performRequest(router, "GET", "/api/v1/leases", tenantToken, nil)
	require.Equal(t, http.StatusOK, w.Code)
	var leases []map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &leases)
	require.Len(t, leases, 1)
	assert.Equal(t, 200.0, leases[0]["charges_amount"]) // Should be 200, not 150
	assert.Equal(t, 900.0, leases[0]["rent_amount"])

	// 6. Verify Charges in HTML Preview (Stable, no Rod needed)
	w = performRequest(router, "GET", "/api/v1/leases/"+strconv.Itoa(leaseID)+"/preview", tenantToken, nil)
	require.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	// Total mensual should be 900 + 200 = 1100
	assert.Contains(t, body, "900.00")  // Rent HC
	assert.Contains(t, body, "200.00")  // Charges
	assert.Contains(t, body, "1100.00") // Total
}

func login(t *testing.T, email, password string) string {
	w := performRequest(router, "POST", "/api/v1/auth/login", "", map[string]string{
		"email": email, "password": password,
	})
	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	return resp["token"].(string)
}
