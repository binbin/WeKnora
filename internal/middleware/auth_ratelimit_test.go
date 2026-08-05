package middleware

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"

	"github.com/Tencent/WeKnora/internal/ratelimit"
)

func setupTestRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return mr, client
}

func TestAuthRateLimit_Disabled(t *testing.T) {
	// Save original values
	origMinute := authRateLimitPerMinute
	origHour := authRateLimitPerHour
	defer func() {
		authRateLimitPerMinute = origMinute
		authRateLimitPerHour = origHour
	}()

	// Disable rate limiting
	authRateLimitPerMinute = 0
	authRateLimitPerHour = 0

	// When disabled, AuthRateLimit should return a pass-through middleware
	// We just verify it doesn't panic with nil redis
	assert.NotPanics(t, func() {
		_ = AuthRateLimit(nil)
	})
}

func TestAuthRateLimit_MiddlewareIntegration(t *testing.T) {
	mr, client := setupTestRedis(t)
	defer mr.Close()

	// Save original values
	origMinute := authRateLimitPerMinute
	origHour := authRateLimitPerHour
	defer func() {
		authRateLimitPerMinute = origMinute
		authRateLimitPerHour = origHour
	}()

	// Set low limits for testing
	authRateLimitPerMinute = 3
	authRateLimitPerHour = 100

	gin.SetMode(gin.TestMode)
	r := gin.New()
	_ = r.SetTrustedProxies([]string{"0.0.0.0/0", "::/0"})
	r.Use(AuthRateLimit(client))
	r.GET("/test", func(c *gin.Context) {
		c.String(200, "ok")
	})

	// First 3 requests should succeed
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Forwarded-For", "192.168.1.1")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code, "Request %d should succeed", i+1)
	}

	// 4th request should be rate limited
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Forwarded-For", "192.168.1.1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, 429, w.Code)
}

func TestAuthRateLimit_LimiterIntegration(t *testing.T) {
	mr, client := setupTestRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// Create a limiter with a small window and low max
	minuteLimiter := ratelimit.New(client, "test:ratelimit:minute:", time.Minute, "test-instance")

	// First 3 requests should be allowed
	for i := 0; i < 3; i++ {
		allowed := minuteLimiter.Allow(ctx, "192.168.1.1", 3)
		assert.True(t, allowed, "Request %d should be allowed", i+1)
	}

	// 4th request should be denied
	allowed := minuteLimiter.Allow(ctx, "192.168.1.1", 3)
	assert.False(t, allowed, "4th request should be denied")
}

func TestAuthRateLimit_DifferentIPs(t *testing.T) {
	mr, client := setupTestRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// Create a limiter
	minuteLimiter := ratelimit.New(client, "test:ratelimit:minute:", time.Minute, "test-instance")

	// Exhaust limit for IP1 (max 2)
	for i := 0; i < 2; i++ {
		allowed := minuteLimiter.Allow(ctx, "192.168.1.1", 2)
		assert.True(t, allowed, "IP1 request %d should be allowed", i+1)
	}

	// IP1 should be rate limited
	allowed := minuteLimiter.Allow(ctx, "192.168.1.1", 2)
	assert.False(t, allowed, "IP1 should be rate limited")

	// IP2 should still be allowed
	allowed = minuteLimiter.Allow(ctx, "192.168.1.2", 2)
	assert.True(t, allowed, "IP2 should be allowed")
}

func TestLoginRateLimit_LimiterIntegration(t *testing.T) {
	mr, client := setupTestRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// Login limiter has stricter limits (5/minute)
	loginLimiter := ratelimit.New(client, "test:login:ratelimit:minute:", time.Minute, "test-instance")

	// First 5 requests should succeed
	for i := 0; i < 5; i++ {
		allowed := loginLimiter.Allow(ctx, "10.0.0.1", 5)
		assert.True(t, allowed, "Login request %d should be allowed", i+1)
	}

	// 6th request should be rate limited
	allowed := loginLimiter.Allow(ctx, "10.0.0.1", 5)
	assert.False(t, allowed, "Login should be rate limited after 5 attempts")
}

func TestAuthRateLimit_ZeroMaxAllowsAll(t *testing.T) {
	mr, client := setupTestRedis(t)
	defer mr.Close()

	ctx := context.Background()

	limiter := ratelimit.New(client, "test:ratelimit:", time.Minute, "test-instance")

	// When max <= 0, should always allow
	for i := 0; i < 100; i++ {
		allowed := limiter.Allow(ctx, "any-ip", 0)
		assert.True(t, allowed, "Should always allow when max=0")
	}
}
