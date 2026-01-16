package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestE2E_LeaseStorageOnAccept(t *testing.T) {
	// 1. Get Configured Storage (from TestMain)
	tempStorageDir := viper.GetString("STORAGE_DIR")
	require.NotEmpty(t, tempStorageDir, "STORAGE_DIR must be set by TestMain")

	// Need ASSETS_DIR for templates
	viper.Set("ASSETS_DIR", "../../assets")
	defer viper.Set("ASSETS_DIR", "")

	// 2. Setup Data
	ownerEmail := "owner_" + randomString() + "@test.com"
	tenantEmail := "tenant_" + randomString() + "@test.com"

	// Register Owner
	w := performRequest(router, "POST", "/api/v1/auth/register", "", map[string]string{
		"email": ownerEmail, "password": "password", "first_name": "Owner", "last_name": "Test", "phone": "123",
	})
	require.Equal(t, http.StatusCreated, w.Code)

	// Login Owner
	w = performRequest(router, "POST", "/api/v1/auth/login", "", map[string]string{"email": ownerEmail, "password": "password"})
	require.Equal(t, http.StatusOK, w.Code)
	var loginResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &loginResp)
	ownerToken := loginResp["token"].(string)

	// Subscribe
	performRequest(router, "POST", "/api/v1/subscriptions", ownerToken, map[string]string{"plan": "discovery", "frequency": "monthly"})

	// Create Property
	w = performRequest(router, "POST", "/api/v1/properties", ownerToken, map[string]interface{}{
		"address": "Storage Test St", "rental_type": "long_term", "details": map[string]string{},
		"rent_amount": 1000, "is_furnished": false,
	})
	require.Equal(t, http.StatusCreated, w.Code)
	var propResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &propResp)
	propID := int(propResp["id"].(float64))

	// 3. Invite Tenant
	w = performRequest(router, "POST", "/api/v1/invitations", ownerToken, map[string]interface{}{
		"property_id": propID, "email": tenantEmail,
	})
	require.Equal(t, http.StatusCreated, w.Code)
	var invResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &invResp)
	token := invResp["token"].(string)
	require.NotEmpty(t, token)

	// 4. Register Tenant (Accepts Invite)
	w = performRequest(router, "POST", "/api/v1/auth/register", "", map[string]interface{}{
		"email": tenantEmail, "password": "password", "first_name": "Tenant", "last_name": "Test", "phone": "456",
		"invite_token": token,
	})
	require.Equal(t, http.StatusCreated, w.Code)

	// 5. Verify Lease Created in DB
	var leaseID int
	err := pool.QueryRow(context.Background(), "SELECT id FROM leases WHERE property_id = $1", propID).Scan(&leaseID)
	require.NoError(t, err)
	require.NotZero(t, leaseID)

	// 6. Verify File Exists in Storage
	expectedFilename := fmt.Sprintf("lease_%d.html", leaseID)
	expectedPath := filepath.Join(tempStorageDir, expectedFilename)

	// Wait a moment as generation is sync but maybe OS flush?
	// It's sync in code.

	_, err = os.Stat(expectedPath)
	assert.NoError(t, err, "Lease file should exist in storage")

	// 7. Verify Download works (should use stored file)
	// Login Tenant
	w = performRequest(router, "POST", "/api/v1/auth/login", "", map[string]string{"email": tenantEmail, "password": "password"})
	var tenLogin map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &tenLogin)
	tenToken := tenLogin["token"].(string)

	w = performRequest(router, "GET", "/api/v1/leases/"+strconv.Itoa(leaseID)+"/download", tenToken, nil)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "CONTRAT DE LOCATION")
}
