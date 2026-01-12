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
		"email": email, "password": "password123", "first_name": "Test", "last_name": "User", "phone_number": "123",
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

	// 3. Subscribe
	w = performRequest(router, "POST", "/api/v1/subscriptions", token, map[string]string{
		"plan": "discovery", "frequency": "monthly",
	})
	require.Equal(t, http.StatusCreated, w.Code)

	// 4. Create Property - Seasonal (Unlimited)
	w = performRequest(router, "POST", "/api/v1/properties", token, map[string]interface{}{
		"address": "Seasonal 1", "rental_type": "seasonal", "details": map[string]string{},
	})
	require.Equal(t, http.StatusCreated, w.Code)

	var propResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &propResp)

	var propID int
	if pid, ok := propResp["property_id"].(float64); ok {
		propID = int(pid)
	}
	require.NotZero(t, propID)

	// 5. Buy Credits
	w = performRequest(router, "POST", "/api/v1/solvency/credits", token, map[string]string{"pack_type": "pack_20"})
	require.Equal(t, http.StatusOK, w.Code)

	// 6. Solvency Check
	w = performRequest(router, "POST", "/api/v1/solvency/check", token, map[string]interface{}{
		"property_id": propID, "candidate_email": "t@t.com",
	})
	require.Equal(t, http.StatusCreated, w.Code)
}

func TestE2E_ErrorCases(t *testing.T) {
	email := getEmail()

	// 1. Auth Failures (Wrong Password)
	// We haven't registered this email yet, so it should fail user not found (401)
	w := performRequest(router, "POST", "/api/v1/auth/login", "", map[string]string{"email": email, "password": "wrong"})
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// Register User
	w = performRequest(router, "POST", "/api/v1/auth/register", "", map[string]string{
		"email": email, "password": "password123", "first_name": "Err", "last_name": "User", "phone_number": "000",
	})
	require.Equal(t, http.StatusCreated, w.Code)

	// Login to get token
	w = performRequest(router, "POST", "/api/v1/auth/login", "", map[string]string{"email": email, "password": "password123"})
	require.Equal(t, http.StatusOK, w.Code)

	var loginResp map[string]string
	json.Unmarshal(w.Body.Bytes(), &loginResp)
	token := loginResp["token"]
	require.NotEmpty(t, token)

	// 2. Unauthenticated Access
	w = performRequest(router, "POST", "/api/v1/properties", "", nil)
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// 3. Subscription: Subscribe with invalid plan
	w = performRequest(router, "POST", "/api/v1/subscriptions", token, map[string]string{"plan": "mega_ultra_pro"})
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// SUBSCRIBE (Required for Property Creation)
	w = performRequest(router, "POST", "/api/v1/subscriptions", token, map[string]string{"plan": "discovery", "frequency": "monthly"})
	require.Equal(t, http.StatusCreated, w.Code)

	// 4. Solvency: Insufficient Funds
	// Create Seasonal Property first (Unlimited quota)
	w = performRequest(router, "POST", "/api/v1/properties", token, map[string]interface{}{
		"address": "Seasonal Err", "rental_type": "seasonal", "details": map[string]string{},
	})
	require.Equal(t, http.StatusCreated, w.Code)

	var propResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &propResp)

	var propID int
	if pid, ok := propResp["property_id"].(float64); ok {
		propID = int(pid)
	}
	require.NotZero(t, propID)

	w = performRequest(router, "POST", "/api/v1/solvency/check", token, map[string]interface{}{
		"property_id": propID, "candidate_email": "broke@test.com",
	})
	// Expect 400 with "insufficient credits" message
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "insufficient credits")

	// 5. Property Quota Exceeded (Discovery Plan)
	// Try to create Long Term property (Quota Exceeded for Discovery)
	w = performRequest(router, "POST", "/api/v1/properties", token, map[string]interface{}{
		"address": "Long Term Err", "rental_type": "long_term", "details": map[string]string{},
	})
	// Expect 400 Bad Request
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "quota exceeded")
}
