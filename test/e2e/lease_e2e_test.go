package e2e

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestE2E_ListLeases(t *testing.T) {
	// 1. Create Users
	// Tenant who will have a lease
	tenantEmail := "tenant_" + randomString() + "@example.com"
	tenantToken := registerAndLogin(t, tenantEmail, "Tenant", "One")

	// Stranger who should not see the lease
	strangerEmail := "stranger_" + randomString() + "@example.com"
	strangerToken := registerAndLogin(t, strangerEmail, "Stranger", "One")

	// Owner (needed to create property)
	ownerEmail := "owner_" + randomString() + "@example.com"
	registerAndLogin(t, ownerEmail, "Owner", "One")
	// We need owner ID to insert property
	var ownerID int
	err := pool.QueryRow(context.Background(), "SELECT id FROM users WHERE email=$1", ownerEmail).Scan(&ownerID)
	require.NoError(t, err)

	// Get Tenant ID
	var tenantID int
	err = pool.QueryRow(context.Background(), "SELECT id FROM users WHERE email=$1", tenantEmail).Scan(&tenantID)
	require.NoError(t, err)

	// 2. Insert Property (Direct DB)
	var propertyID int
	err = pool.QueryRow(context.Background(), `
		INSERT INTO properties (owner_id, address, rental_type, created_at)
		VALUES ($1, '123 Lease St', 'long_term', NOW())
		RETURNING id
	`, ownerID).Scan(&propertyID)
	require.NoError(t, err)

	// 3. Insert Lease (Direct DB)
	_, err = pool.Exec(context.Background(), `
		INSERT INTO leases (
			property_id, tenant_id, start_date, rent_amount, deposit_amount, lease_status, created_at
		) VALUES (
			$1, $2, NOW(), 1000, 2000, 'active', NOW()
		)
	`, propertyID, tenantID)
	require.NoError(t, err)

	t.Run("Nominal Case: Tenant lists their leases", func(t *testing.T) {
		w := performRequest(router, "GET", "/api/v1/leases", tenantToken, nil)

		require.Equal(t, http.StatusOK, w.Code)

		var leases []map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &leases)
		require.NoError(t, err)

		assert.Len(t, leases, 1)
		// Handle float64/int conversion from JSON unmarshal
		returnedPropID, _ := leases[0]["property_id"].(float64)
		assert.Equal(t, float64(propertyID), returnedPropID)
		assert.Equal(t, "123 Lease St", leases[0]["property_address"])
	})

	t.Run("Empty Case: Stranger has no leases", func(t *testing.T) {
		w := performRequest(router, "GET", "/api/v1/leases", strangerToken, nil)

		require.Equal(t, http.StatusOK, w.Code)

		var leases []map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &leases)
		require.NoError(t, err)

		assert.Len(t, leases, 0)
	})

	t.Run("Unauthorized", func(t *testing.T) {
		w := performRequest(router, "GET", "/api/v1/leases", "", nil)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

// Helper local to this file to avoid modifying e2e_test.go
func registerAndLogin(t *testing.T, email, firstName, lastName string) string {
	// Register
	w := performRequest(router, "POST", "/api/v1/auth/register", "", map[string]string{
		"email": email, "password": "password123", "first_name": firstName, "last_name": lastName, "phone": "1234567890",
	})
	require.Equal(t, http.StatusCreated, w.Code)

	// Login
	w = performRequest(router, "POST", "/api/v1/auth/login", "", map[string]string{
		"email": email, "password": "password123",
	})
	require.Equal(t, http.StatusOK, w.Code)

	var loginResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &loginResp)
	token, ok := loginResp["token"].(string)
	require.True(t, ok)
	return token
}
