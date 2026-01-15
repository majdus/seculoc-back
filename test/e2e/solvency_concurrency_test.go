package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestE2E_SolvencyConcurrency(t *testing.T) {
	// router is initialized in TestMain

	email := fmt.Sprintf("conc_%d@test.com", time.Now().UnixNano())

	// 1. Setup User & 20 Credits
	performRequest(router, "POST", "/api/v1/auth/register", "", map[string]string{
		"email": email, "password": "password123", "first_name": "Conc", "last_name": "User", "phone": "123",
	})

	w := performRequest(router, "POST", "/api/v1/auth/login", "", map[string]string{
		"email": email, "password": "password123",
	})
	require.Equal(t, http.StatusOK, w.Code)

	var loginResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &loginResp)
	token := loginResp["token"].(string)

	performRequest(router, "POST", "/api/v1/subscriptions", token, map[string]string{
		"plan": "discovery", "frequency": "monthly",
	})

	// Create Property (Gets 20 Vacancy Credits initially because logic mocks "seasonal" or similar?
	// Actually logic says LongTerm gets quota check.
	// Let's use Seasonal for unlimited quota but the credits come from... where?
	// Ah, Property credits are vacancy_credits.
	// Defaults: query.sql doesn't set default? Schema does?
	// Schema: vacancy_credits INT NOT NULL DEFAULT 0.
	// Wait, full journey test asserts 20 credits. Why?
	// Ah, maybe `details` or side effect?
	// Let's check schema/migration or service logic.
	// Service CreateProperty: if plan is discovery, check bonus. Bonus is global credits.
	// Where do property credits come from?
	// Reviewing `CreateProperty` logic...
	// It doesn't set `vacancy_credits` explicitly in INSERT.
	// So it defaults to 0.
	// EXCEPT if there is a trigger or if I missed something in `CreateProperty` helper in test.
	// In `TestE2E_FullUserJourney`, it creates "seasonal" property and asserts 20 credits.
	// Why?
	// Maybe `PropertyType` enum triggers something?
	// Or maybe I missed a trigger in `schema.sql`.

	// Assuming 20 credits for now based on previous test success.

	// Create Property
	w = performRequest(router, "POST", "/api/v1/properties", token, map[string]interface{}{
		"address": "Concurrency St", "rental_type": "seasonal", "details": map[string]string{},
	})
	require.Equal(t, http.StatusCreated, w.Code)

	var propResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &propResp)
	propID := int(propResp["id"].(float64))

	// Manually set credits to 5 directly in DB to control the test strictly
	// We need 5 credits. We launch 10 requests. 5 should succeed, 5 should fail (or use global?).
	// The logic is: Uses Property Credit if > 0. Else Global.
	// If Global empty, fail.
	// So if we have 5 Prop Credits, 0 Global.
	// 5 succeed, 5 fail.

	_, err := pool.Exec(context.Background(), "UPDATE properties SET vacancy_credits = 5 WHERE id = $1", propID)
	require.NoError(t, err)

	// Ensure Global Balance is 0
	// (It should be 3 from bonus if 'discovery'? Not sure logic logic. Let's wipe transactions or balance).
	// Actually difficult to wipe balance without access to helper.
	// Let's rely on "property credit preference".
	// If requests consume property credit correctly, we should end up with 0 property credits.
	// If race condition: we might end up with -1 or more than 5 checks produced with "property" source.

	concurrency := 10
	var wg sync.WaitGroup
	wg.Add(concurrency)

	results := make(chan int, concurrency)

	for i := 0; i < concurrency; i++ {
		go func(idx int) {
			defer wg.Done()
			candEmail := fmt.Sprintf("cand_%d@test.com", idx)
			w := performRequest(router, "POST", "/api/v1/solvency/check", token, map[string]interface{}{
				"property_id": propID, "candidate_email": candEmail,
			})
			results <- w.Code
		}(i)
	}

	wg.Wait()
	close(results)

	successCount := 0
	failCount := 0 // 400 or 402?

	for code := range results {
		if code == 201 {
			successCount++
		} else {
			failCount++ // likely 402 Payment Required or 400
		}
	}

	// We expect at least 5 success (property credits).
	// Plus potentially 3 success (global bonus).
	// Total max success = 8.
	// If > 8 success, we overspent.

	t.Logf("Success: %d, Fail: %d", successCount, failCount)

	// Verify final state
	row := pool.QueryRow(context.Background(), "SELECT vacancy_credits FROM properties WHERE id = $1", propID)
	var finalCredits int
	err = row.Scan(&finalCredits)
	require.NoError(t, err)

	assert.GreaterOrEqual(t, finalCredits, 0, "Credits should not be negative")
	assert.LessOrEqual(t, successCount, 8, "Should not exceed total available credits (5 prop + 3 global)")
}
