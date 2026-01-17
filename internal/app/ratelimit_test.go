package app

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/ulule/limiter/v3"
	mgin "github.com/ulule/limiter/v3/drivers/middleware/gin"
	"github.com/ulule/limiter/v3/drivers/store/memory"
)

func TestRateLimit_Integration(t *testing.T) {
	// Setup
	gin.SetMode(gin.TestMode)
	r := gin.New()

	// Apply middleware manually to bypass the GIN_MODE=test check in app.go
	rate := limiter.Rate{Period: 1 * time.Second, Limit: 20}
	store := memory.NewStore()
	instance := limiter.New(store, rate)
	r.Use(mgin.NewMiddleware(instance))

	r.GET("/ping", func(c *gin.Context) {
		c.String(200, "pong")
	})

	t.Run("Nominal Case - Under Limit", func(t *testing.T) {
		// Limit is 20 req/s. Sending 5 should be fine.
		for i := 0; i < 5; i++ {
			req := httptest.NewRequest("GET", "/ping", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code, "Request %d should pass", i)
		}
	})

	t.Run("Edge Case - Exceed Limit", func(t *testing.T) {
		r2 := gin.New()

		// Manual setup
		rate := limiter.Rate{Period: 1 * time.Second, Limit: 20}
		store := memory.NewStore()
		instance := limiter.New(store, rate)
		r2.Use(mgin.NewMiddleware(instance))

		r2.GET("/ping", func(c *gin.Context) { c.Status(200) })

		// Send 20 requests (Limit)
		for i := 0; i < 20; i++ {
			req := httptest.NewRequest("GET", "/ping", nil)
			w := httptest.NewRecorder()
			r2.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
		}

		// Send 21st request - Should be blocked
		req := httptest.NewRequest("GET", "/ping", nil)
		w := httptest.NewRecorder()
		r2.ServeHTTP(w, req)
		// Usually returns 429
		assert.Equal(t, http.StatusTooManyRequests, w.Code)
	})

	t.Run("Nominal Case - Recovery", func(t *testing.T) {
		r3 := gin.New()

		// Manual setup
		rate := limiter.Rate{Period: 1 * time.Second, Limit: 20}
		store := memory.NewStore()
		instance := limiter.New(store, rate)
		r3.Use(mgin.NewMiddleware(instance))

		r3.GET("/ping", func(c *gin.Context) { c.Status(200) })

		// Exhaust limit
		for i := 0; i < 20; i++ {
			r3.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/ping", nil))
		}

		// Verify Blocked
		wBlocked := httptest.NewRecorder()
		r3.ServeHTTP(wBlocked, httptest.NewRequest("GET", "/ping", nil))
		assert.Equal(t, http.StatusTooManyRequests, wBlocked.Code)

		// Wait
		time.Sleep(1100 * time.Millisecond)

		// Verify Recovered
		wRecovered := httptest.NewRecorder()
		r3.ServeHTTP(wRecovered, httptest.NewRequest("GET", "/ping", nil))
		assert.Equal(t, http.StatusOK, wRecovered.Code)
	})
}
