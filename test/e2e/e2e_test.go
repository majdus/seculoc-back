package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"seculoc-back/internal/app"
	"seculoc-back/internal/platform/logger"
)

var (
	router *gin.Engine
	pool   *pgxpool.Pool
	dbURL  string
)

func TestMain(m *testing.M) {
	ctx := context.Background()

	// 1. Start Postgres Container
	cwd, _ := os.Getwd()
	schemaPath := filepath.Join(cwd, "../../db/schemas.sql")

	pgContainer, err := postgres.Run(ctx,
		"postgres:15-alpine",
		postgres.WithDatabase("seculoc_test"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		postgres.WithInitScripts(schemaPath),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(10*time.Second)),
	)
	if err != nil {
		fmt.Printf("Failed to start container: %v\n", err)
		os.Exit(1)
	}

	dbURL, err = pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		fmt.Printf("Failed to get connection string: %v\n", err)
		os.Exit(1)
	}

	// 2. Setup App Config
	viper.Set("JWT_SECRET", "test_secret_for_e2e")
	viper.Set("ENV", "test")

	// Create temp storage for E2E
	storageDir, _ := os.MkdirTemp("", "e2e_storage")
	defer os.RemoveAll(storageDir)
	viper.Set("STORAGE_DIR", storageDir)

	logger.Init("test")
	log := logger.Get()

	// 3. Connect DB
	poolConfig, _ := pgxpool.ParseConfig(dbURL)
	pool, err = pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		fmt.Printf("Failed to connect to container DB: %v\n", err)
		os.Exit(1)
	}

	// 4. Init App
	router = app.NewServer(pool, log)

	// Run
	code := m.Run()

	// Cleanup
	pgContainer.Terminate(ctx)
	os.Exit(code)
}

// Helpers
func toJson(v interface{}) []byte {
	b, _ := json.Marshal(v)
	return b
}

func randomString() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func performRequest(r *gin.Engine, method, path, token string, body interface{}) *httptest.ResponseRecorder {
	reqBody := bytes.NewBuffer(toJson(body))
	req, _ := http.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func getEmail() string {
	return fmt.Sprintf("e2e_%d@example.com", time.Now().UnixNano())
}

// --- Scenarios ---

func TestE2E_FullUserJourney(t *testing.T) {
	email := getEmail()

	// 1. Register
	w := performRequest(router, "POST", "/api/v1/auth/register", "", map[string]string{
		"email": email, "password": "password123", "first_name": "John", "last_name": "Doe", "phone": "1234567890",
	})
	require.Equal(t, http.StatusCreated, w.Code)

	// 2. Login
	w = performRequest(router, "POST", "/api/v1/auth/login", "", map[string]string{
		"email": email, "password": "password123",
	})
	require.Equal(t, http.StatusOK, w.Code)

	var loginResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &loginResp)
	token := loginResp["token"].(string)
	require.NotEmpty(t, token)

	// Assert User Profile
	userInfo, ok := loginResp["user"].(map[string]interface{})
	require.True(t, ok, "user object missing in login response")
	profile, ok := userInfo["owner_profile"].(map[string]interface{})
	require.True(t, ok, "owner_profile missing in user object")
	// Check credit balance (default 0 or null)
	balance, _ := profile["credit_balance"].(float64)
	assert.Equal(t, float64(0), balance)

	// 3. Subscribe
	w = performRequest(router, "POST", "/api/v1/subscriptions", token, map[string]string{
		"plan": "discovery", "frequency": "monthly",
	})
	require.Equal(t, http.StatusOK, w.Code)

	var subResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &subResp)
	subData, ok := subResp["data"].(map[string]interface{})
	require.True(t, ok, "data object missing in subscription response")
	subUser, ok := subData["user"].(map[string]interface{})
	require.True(t, ok, "user object missing in subscription response data")
	subProfile, ok := subUser["owner_profile"].(map[string]interface{})
	require.True(t, ok, "owner_profile missing in subscription response user")
	subDetails, ok := subProfile["subscription"].(map[string]interface{})
	require.True(t, ok, "subscription details missing in profile")
	assert.Equal(t, "discovery", subDetails["plan_type"])

	// 4. Create Property - Seasonal (Unlimited)
	w = performRequest(router, "POST", "/api/v1/properties", token, map[string]interface{}{
		"address": "Seasonal 1", "rental_type": "seasonal", "details": map[string]string{},
		"rent_amount": 1000, "rent_charges_amount": 100, "is_furnished": true, "seasonal_price_per_night": 50,
	})
	require.Equal(t, http.StatusCreated, w.Code)

	var propResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &propResp)
	t.Logf("FullJourney Prop Response: %+v", propResp)

	var propID int
	if v, ok := propResp["id"]; ok {
		switch val := v.(type) {
		case float64:
			propID = int(val)
		case int:
			propID = val
		default:
			t.Logf("Unexpected type: %T", val)
		}
	}
	require.NotZero(t, propID, "Property ID missing in FullUserJourney")

	// Verify Vacancy Credits
	if vc, ok := propResp["vacancy_credits"].(float64); ok {
		assert.Equal(t, float64(20), vc, "Expected 20 vacancy credits")
	} else {
		assert.Fail(t, "vacancy_credits missing in FullUserJourney")
	}

	// 5. Buy Credits
	w = performRequest(router, "POST", "/api/v1/solvency/credits", token, map[string]string{"pack_type": "pack_20"})
	require.Equal(t, http.StatusOK, w.Code)

	// 6. Solvency Check (Consumes Property Credit)
	w = performRequest(router, "POST", "/api/v1/solvency/check", token, map[string]interface{}{
		"property_id": propID, "candidate_email": "t@t.com",
	})
	require.Equal(t, http.StatusCreated, w.Code)

	// 7. Verify Property Credit Decremented (20 -> 19)
	w = performRequest(router, "GET", "/api/v1/properties", token, nil)
	require.Equal(t, http.StatusOK, w.Code)
	var props []map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &props)
	require.NotEmpty(t, props)
	// Find our property
	found := false
	for _, p := range props {
		if int(p["id"].(float64)) == propID {
			vc := int(p["vacancy_credits"].(float64))
			assert.Equal(t, 19, vc, "Expected vacancy credits to decrease to 19")
			found = true
			break
		}
	}
	require.True(t, found, "Property not found in list")
}

func TestE2E_ErrorCases(t *testing.T) {
	email := getEmail()

	// 1. Auth Failures (Wrong Password)
	// We haven't registered this email yet, so it should fail user not found (401)
	w := performRequest(router, "POST", "/api/v1/auth/login", "", map[string]string{"email": email, "password": "wrong"})
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// Register User
	w = performRequest(router, "POST", "/api/v1/auth/register", "", map[string]string{
		"email": email, "password": "password123", "first_name": "Err", "last_name": "User", "phone": "000",
	})
	require.Equal(t, http.StatusCreated, w.Code)

	// Login to get token
	w = performRequest(router, "POST", "/api/v1/auth/login", "", map[string]string{"email": email, "password": "password123"})
	require.Equal(t, http.StatusOK, w.Code)

	var loginResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &loginResp)
	token := loginResp["token"].(string)
	require.NotEmpty(t, token)

	// 2. Unauthenticated Access
	w = performRequest(router, "POST", "/api/v1/properties", "", nil)
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// 3. Subscription: Subscribe with invalid plan
	w = performRequest(router, "POST", "/api/v1/subscriptions", token, map[string]string{"plan": "mega_ultra_pro"})
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// SUBSCRIBE (Required for Property Creation)
	w = performRequest(router, "POST", "/api/v1/subscriptions", token, map[string]string{"plan": "discovery", "frequency": "monthly"})
	require.Equal(t, http.StatusOK, w.Code)

	// 4. Solvency: Insufficient Funds
	// Create Seasonal Property first (Unlimited quota)
	w = performRequest(router, "POST", "/api/v1/properties", token, map[string]interface{}{
		"address": "Seasonal Err", "rental_type": "seasonal", "details": map[string]string{},
	})
	require.Equal(t, http.StatusCreated, w.Code)

	var propResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &propResp)
	t.Logf("DEBUG PROPERTY RESPONSE: %+v", propResp)

	var propID int
	if v, ok := propResp["id"]; ok {
		switch val := v.(type) {
		case float64:
			propID = int(val)
		case int:
			propID = val
		default:
			t.Logf("Unexpected ID type: %T", val)
		}
	} else {
		t.Logf("ID missing in response")
	}
	require.NotZero(t, propID, "Property ID should be present")

	// Verify Vacancy Credits
	if vc, ok := propResp["vacancy_credits"].(float64); ok {
		assert.Equal(t, float64(20), vc, "Expected 20 vacancy credits")
	} else {
		assert.Fail(t, "vacancy_credits missing in response")
	}

	// Manually set vacancy credits to 0 to test insufficient funds
	_, err := pool.Exec(context.Background(), "UPDATE properties SET vacancy_credits = 0 WHERE id = $1", propID)
	require.NoError(t, err)

	w = performRequest(router, "POST", "/api/v1/solvency/check", token, map[string]interface{}{
		"property_id": propID, "candidate_email": "broke@test.com",
	})
	// Expect 400 with "insufficient credits" message
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "insufficient credits")

	// 5. Property Quota Exceeded (Discovery Plan limit is 1 Long Term)
	// First Long Term Property -> Should Succeed (1/1)
	w = performRequest(router, "POST", "/api/v1/properties", token, map[string]interface{}{
		"address": "Long Term 1", "rental_type": "long_term", "details": map[string]string{},
	})
	require.Equal(t, http.StatusCreated, w.Code)

	// Second Long Term Property -> Should Fail (2/1)
	w = performRequest(router, "POST", "/api/v1/properties", token, map[string]interface{}{
		"address": "Long Term 2", "rental_type": "long_term", "details": map[string]string{},
	})
	// Expect 400 Bad Request
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "quota exceeded")
}

func TestE2E_StickyState(t *testing.T) {
	// 1. Register (Default Owner)
	email := "sticky_" + randomString() + "@example.com"
	performRequest(router, "POST", "/api/v1/auth/register", "", map[string]interface{}{
		"email": email, "password": "password123", "first_name": "Sticky", "last_name": "User", "phone": "123",
	})

	// 2. Login -> Should be Owner (Default)
	w := performRequest(router, "POST", "/api/v1/auth/login", "", map[string]interface{}{
		"email": email, "password": "password123",
	})
	require.Equal(t, http.StatusOK, w.Code)
	var loginResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &loginResp)
	token := loginResp["token"].(string)
	currentCtx := loginResp["current_context"].(string)
	assert.Equal(t, "owner", currentCtx)

	// Prerequisite: To switch to Tenant, user needs to be a tenant (have lease).
	// We'll insert a dummy property and lease directly into DB for this user.
	// 1. Get User ID
	var userID int
	// loginResp["user"]["id"] float64...
	userMap := loginResp["user"].(map[string]interface{})
	userID = int(userMap["id"].(float64))

	// 2. Insert Property (Owner can be anyone, let's say same user)
	queryProp := `INSERT INTO properties (owner_id, address, rental_type, created_at) VALUES ($1, 'Stick St', 'long_term', NOW()) RETURNING id`
	var propID int
	err := pool.QueryRow(context.Background(), queryProp, userID).Scan(&propID)
	require.NoError(t, err)

	// 3. Insert Lease
	queryLease := `INSERT INTO leases (property_id, tenant_id, start_date, rent_amount, deposit_amount, lease_status, created_at) 
				   VALUES ($1, $2, NOW(), 500, 1000, 'active', NOW())`
	_, err = pool.Exec(context.Background(), queryLease, propID, userID)
	require.NoError(t, err)

	// 3. Switch Context to Tenant
	w = performRequest(router, "POST", "/api/v1/auth/switch-context", token, map[string]interface{}{
		"target_context": "tenant",
	})
	require.Equal(t, http.StatusOK, w.Code)

	// Verify Response Structure (New Token, Context)
	var switchResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &switchResp)

	newToken, ok := switchResp["token"].(string)
	require.True(t, ok, "New token missing in switch response")
	require.NotEmpty(t, newToken)
	require.NotEqual(t, token, newToken, "Token should change (or at least be a valid new one)")

	newContext, ok := switchResp["current_context"].(string)
	require.True(t, ok)
	assert.Equal(t, "tenant", newContext)

	// 4. Switch Back to Owner
	w = performRequest(router, "POST", "/api/v1/auth/switch-context", newToken, map[string]interface{}{
		"target_context": "owner",
	})
	require.Equal(t, http.StatusOK, w.Code)

	json.Unmarshal(w.Body.Bytes(), &switchResp)
	ownerContext, _ := switchResp["current_context"].(string)
	assert.Equal(t, "owner", ownerContext)
}
