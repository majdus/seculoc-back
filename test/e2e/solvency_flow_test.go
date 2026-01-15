package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestE2E_SolvencyFlow(t *testing.T) {
	ownerEmail := getEmail()
	candidateEmail := "candidate_" + randomString() + "@example.com"

	// 1. Register Owner
	performRequest(router, "POST", "/api/v1/auth/register", "", map[string]string{
		"email": ownerEmail, "password": "password123", "first_name": "Prosper", "last_name": "Landlord", "phone": "123",
	})

	// 2. Login Owner
	w := performRequest(router, "POST", "/api/v1/auth/login", "", map[string]string{
		"email": ownerEmail, "password": "password123",
	})
	require.Equal(t, http.StatusOK, w.Code)
	var loginResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &loginResp)
	ownerToken := loginResp["token"].(string)

	// 3. Subscribe & Create Property
	performRequest(router, "POST", "/api/v1/subscriptions", ownerToken, map[string]string{
		"plan": "discovery", "frequency": "monthly",
	})
	// Refresh token for credits
	w = performRequest(router, "POST", "/api/v1/auth/login", "", map[string]string{
		"email": ownerEmail, "password": "password123",
	})
	json.Unmarshal(w.Body.Bytes(), &loginResp)
	ownerToken = loginResp["token"].(string)

	w = performRequest(router, "POST", "/api/v1/properties", ownerToken, map[string]interface{}{
		"name":           "Luxury Penthouse",
		"address":        "Solvency Palace",
		"rental_type":    "long_term",
		"details":        map[string]interface{}{"surface": 50},
		"rent_amount":    1000,
		"deposit_amount": 2000,
	})
	require.Equal(t, http.StatusCreated, w.Code)
	var propResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &propResp)
	propID := int(propResp["id"].(float64))

	// 4. Owner Initiates Solvency Check
	w = performRequest(router, "POST", "/api/v1/solvency/check", ownerToken, map[string]interface{}{
		"property_id":          propID,
		"candidate_email":      candidateEmail,
		"candidate_first_name": "Alice",
		"candidate_last_name":  "Candidate",
	})
	if w.Code != http.StatusCreated {
		t.Logf("Solvency check initiation failed with code %d: %s", w.Code, w.Body.String())
	}
	require.Equal(t, http.StatusCreated, w.Code)
	var checkResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &checkResp)
	assert.NotEmpty(t, checkResp["id"])
	token := checkResp["token"].(string)
	require.NotEmpty(t, token)
	assert.NotEmpty(t, checkResp["verification_url"])

	// Assert List Checks shows token/url
	w = performRequest(router, "GET", "/api/v1/solvency/checks", ownerToken, nil)
	require.Equal(t, http.StatusOK, w.Code)
	var checks []map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &checks)
	found := false
	for _, c := range checks {
		if c["token"] == token {
			assert.NotEmpty(t, c["verification_url"])
			found = true
			break
		}
	}
	assert.True(t, found, "Token not found in history list")

	// 5. Candidate Views Check Details (Public)
	w = performRequest(router, "GET", "/api/v1/solvency/public/check/"+token, "", nil)
	require.Equal(t, http.StatusOK, w.Code)
	var publicResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &publicResp)
	require.Equal(t, candidateEmail, publicResp["candidate_email"])
	require.Equal(t, "pending", publicResp["status"])
	require.Equal(t, "Solvency Palace", publicResp["property_address"], "Property address mismatch")
	require.Equal(t, "Luxury Penthouse", publicResp["property_name"], "Property name mismatch")
	require.Equal(t, 1000.0, publicResp["rent_amount"], "Rent amount mismatch")

	// 6. Mock Open Banking Callback (Success: Income > 3x Rent)
	// Rent is 1000, 3x is 3000. Avg monthly income = sum / 3.
	// We need 9000 total income over 3 months.
	w = performRequest(router, "POST", "/api/v1/solvency/public/check/"+token+"/callback", "", map[string]interface{}{
		"transactions": []map[string]interface{}{
			{"amount": 3000.0, "description": "Salary Jan", "date": "2024-01-01T00:00:00Z"},
			{"amount": 3000.0, "description": "Salary Feb", "date": "2024-02-01T00:00:00Z"},
			{"amount": 3000.0, "description": "Salary Mar", "date": "2024-03-01T00:00:00Z"},
		},
	})
	require.Equal(t, http.StatusOK, w.Code)

	// 7. Verify Result Status
	w = performRequest(router, "GET", "/api/v1/solvency/public/check/"+token, "", nil)
	json.Unmarshal(w.Body.Bytes(), &publicResp)
	require.Equal(t, "approved", publicResp["status"], "Check should be approved after valid salary callback")

	// 8. Mock Open Banking Callback (Failure: Income < 3x Rent)
	// Reuse property (to avoid Free Plan limit of 1 property)
	w = performRequest(router, "POST", "/api/v1/solvency/check", ownerToken, map[string]interface{}{
		"property_id":     propID,
		"candidate_email": "broke@example.com",
	})
	json.Unmarshal(w.Body.Bytes(), &checkResp)
	token2 := checkResp["token"].(string)

	performRequest(router, "POST", "/api/v1/solvency/public/check/"+token2+"/callback", "", map[string]interface{}{
		"transactions": []map[string]interface{}{
			{"amount": 1000.0, "description": "Tiny Jan", "date": "2024-01-01T00:00:00Z"},
		},
	})

	w = performRequest(router, "GET", "/api/v1/solvency/public/check/"+token2, "", nil)
	json.Unmarshal(w.Body.Bytes(), &publicResp)
	assert.Equal(t, "rejected", publicResp["status"])

	// 9. Verify Provisional User Promotion
	candidateEmailProvisional := "prov_" + randomString() + "@example.com"
	// Owner invites provisional
	performRequest(router, "POST", "/api/v1/solvency/check", ownerToken, map[string]interface{}{
		"property_id": propID, "candidate_email": candidateEmailProvisional,
	})

	// Candidate registers (Promotion)
	w = performRequest(router, "POST", "/api/v1/auth/register", "", map[string]string{
		"email": candidateEmailProvisional, "password": "securepassword", "first_name": "Pro", "last_name": "Visional", "phone": "123",
	})
	require.Equal(t, http.StatusCreated, w.Code)

	// Candidate can now login
	w = performRequest(router, "POST", "/api/v1/auth/login", "", map[string]string{
		"email": candidateEmailProvisional, "password": "securepassword",
	})
	assert.Equal(t, http.StatusOK, w.Code)

	// 10. Edge Case: Property Access Denied
	// Create another owner
	otherOwnerEmail := "other_" + getEmail()
	performRequest(router, "POST", "/api/v1/auth/register", "", map[string]string{
		"email": otherOwnerEmail, "password": "password123", "first_name": "Spy", "last_name": "Owner", "phone": "456",
	})
	w = performRequest(router, "POST", "/api/v1/auth/login", "", map[string]string{
		"email": otherOwnerEmail, "password": "password123",
	})
	var otherLoginResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &otherLoginResp)
	otherToken := otherLoginResp["token"].(string)

	// Spy owner tries to initiate check on Prosper's property
	w = performRequest(router, "POST", "/api/v1/solvency/check", otherToken, map[string]interface{}{
		"property_id": propID, "candidate_email": "victim@example.com",
	})
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "property not found or access denied")

	// 11. Edge Case: Double Processing
	// Try to process token2 again (already rejected)
	// 11. Edge Case: Double Processing
	// Try to process token2 again (already rejected)
	w = performRequest(router, "POST", "/api/v1/solvency/public/check/"+token2+"/callback", "", map[string]interface{}{
		"transactions": []map[string]interface{}{},
	})
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "check already processed")

	// 12. Edge Case: Invalid Token
	w = performRequest(router, "GET", "/api/v1/solvency/public/check/invalid_token", "", nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}
func TestE2E_SolvencyCreditFallback(t *testing.T) {
	ownerEmail := "fallback_" + getEmail()

	// 1. Register Owner
	performRequest(router, "POST", "/api/v1/auth/register", "", map[string]string{
		"email": ownerEmail, "password": "password123", "first_name": "Prosper", "last_name": "Landlord", "phone": "123",
	})

	// 2. Login Owner
	w := performRequest(router, "POST", "/api/v1/auth/login", "", map[string]string{
		"email": ownerEmail, "password": "password123",
	})
	var loginResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &loginResp)
	ownerToken := loginResp["token"].(string)

	// 3. Subscribe & Create 1st Property (Grants 3 Global Credits)
	performRequest(router, "POST", "/api/v1/subscriptions", ownerToken, map[string]string{
		"plan": "discovery", "frequency": "monthly",
	})

	w = performRequest(router, "POST", "/api/v1/properties", ownerToken, map[string]interface{}{
		"address": "Property 1", "rental_type": "long_term", "details": map[string]string{},
		"rent_amount": 1000.0, "deposit_amount": 2000.0,
	})
	var propResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &propResp)
	propID1 := int(propResp["id"].(float64))

	// 4. Consume 20 property credits
	for i := 0; i < 20; i++ {
		w = performRequest(router, "POST", "/api/v1/solvency/check", ownerToken, map[string]interface{}{
			"property_id": propID1, "candidate_email": fmt.Sprintf("cand_%d@test.com", i),
		})
		require.Equal(t, http.StatusCreated, w.Code)
	}

	// 5. 21st Check: Should use Global Credits (Bonus has 3)
	w = performRequest(router, "POST", "/api/v1/solvency/check", ownerToken, map[string]interface{}{
		"property_id": propID1, "candidate_email": "global_consumer@test.com",
	})
	assert.Equal(t, http.StatusCreated, w.Code)
	// Check log for "credit_source": "global" manually or just verify success

	// 6. Consume remaining 2 global credits
	for i := 0; i < 2; i++ {
		performRequest(router, "POST", "/api/v1/solvency/check", ownerToken, map[string]interface{}{
			"property_id": propID1, "candidate_email": fmt.Sprintf("global_%d@test.com", i),
		})
	}

	// 7. 24th Check: Should return 402 Payment Required
	w = performRequest(router, "POST", "/api/v1/solvency/check", ownerToken, map[string]interface{}{
		"property_id": propID1, "candidate_email": "bankrupt@test.com",
	})
	assert.Equal(t, http.StatusPaymentRequired, w.Code)
	var errResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &errResp)
	assert.Equal(t, "ERR_INSUFFICIENT_CREDITS", errResp["error"])
	details := errResp["details"].(map[string]interface{})
	assert.Equal(t, 0.0, details["global_balance"])
	assert.Equal(t, 0.0, details["property_balance"])
}

func TestE2E_SolvencyCancellationAndRefund(t *testing.T) {
	ownerEmail := "cancel_" + getEmail()

	// 1. Register & Login Owner
	performRequest(router, "POST", "/api/v1/auth/register", "", map[string]string{
		"email": ownerEmail, "password": "password123", "first_name": "Cancellor", "last_name": "Landlord", "phone": "123",
	})
	w := performRequest(router, "POST", "/api/v1/auth/login", "", map[string]string{
		"email": ownerEmail, "password": "password123",
	})
	var loginResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &loginResp)
	ownerToken := loginResp["token"].(string)

	// 1b. Subscribe (Required to create a property)
	performRequest(router, "POST", "/api/v1/subscriptions", ownerToken, map[string]string{
		"plan": "discovery", "frequency": "monthly",
	})

	// 2. Create Property (Has 20 credits)
	w = performRequest(router, "POST", "/api/v1/properties", ownerToken, map[string]interface{}{
		"address": "Refund Property", "rental_type": "long_term", "details": map[string]string{},
		"rent_amount": 1000.0, "deposit_amount": 2000.0,
	})
	var propResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &propResp)
	propID := int(propResp["id"].(float64))

	// 3. Initiate Check (Uses Property Credit, 20 -> 19)
	w = performRequest(router, "POST", "/api/v1/solvency/check", ownerToken, map[string]interface{}{
		"property_id": propID, "candidate_email": "candidate1@test.com",
	})
	require.Equal(t, http.StatusCreated, w.Code)
	var checkResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &checkResp)
	assert.NotEmpty(t, checkResp["id"])
	assert.NotEmpty(t, checkResp["token"])
	assert.NotEmpty(t, checkResp["verification_url"])
	checkID := int(checkResp["id"].(float64))

	// 4. Cancel Check & Verify Refund
	w = performRequest(router, "POST", fmt.Sprintf("/api/v1/solvency/check/%d/cancel", checkID), ownerToken, nil)
	assert.Equal(t, http.StatusOK, w.Code)

	// 5. Verify Property Credit is back to 20 by creating 20 more checks successfully
	for i := 0; i < 20; i++ {
		w = performRequest(router, "POST", "/api/v1/solvency/check", ownerToken, map[string]interface{}{
			"property_id": propID, "candidate_email": fmt.Sprintf("after_cancel_%d@test.com", i),
		})
		assert.Equal(t, http.StatusCreated, w.Code, "Should be able to create 20 checks after refund")
	}

	// 6. Test Global Refund

	// 21st check (Uses 1/3 global)
	w = performRequest(router, "POST", "/api/v1/solvency/check", ownerToken, map[string]interface{}{
		"property_id": propID, "candidate_email": "global_ref@test.com",
	})
	require.Equal(t, http.StatusCreated, w.Code)
	json.Unmarshal(w.Body.Bytes(), &checkResp)
	globalCheckID := int(checkResp["check_id"].(float64))

	// Cancel Global Check
	w = performRequest(router, "POST", fmt.Sprintf("/api/v1/solvency/check/%d/cancel", globalCheckID), ownerToken, nil)
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify we still have 3 global credits by doing 3 checks
	for i := 0; i < 3; i++ {
		w = performRequest(router, "POST", "/api/v1/solvency/check", ownerToken, map[string]interface{}{
			"property_id": propID, "candidate_email": fmt.Sprintf("global_verify_%d@test.com", i),
		})
		assert.Equal(t, http.StatusCreated, w.Code, "Should have 3 global credits after global refund")
	}

	// 4th global should fail
	w = performRequest(router, "POST", "/api/v1/solvency/check", ownerToken, map[string]interface{}{
		"property_id": propID, "candidate_email": "bankrupt@test.com",
	})
	assert.Equal(t, http.StatusPaymentRequired, w.Code)

	// 7. Edge Case: Unauthorized Cancellation
	// Register a second owner
	otherEmail := "hacker_" + getEmail()
	performRequest(router, "POST", "/api/v1/auth/register", "", map[string]string{
		"email": otherEmail, "password": "password123", "first_name": "Hacker", "last_name": "Doe", "phone": "123",
	})
	w = performRequest(router, "POST", "/api/v1/auth/login", "", map[string]string{
		"email": otherEmail, "password": "password123",
	})
	var otherLoginResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &otherLoginResp)
	otherToken := otherLoginResp["token"].(string)

	// Try to cancel the first owner's check
	w = performRequest(router, "POST", fmt.Sprintf("/api/v1/solvency/check/%d/cancel", checkID), otherToken, nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "unauthorized")
}
