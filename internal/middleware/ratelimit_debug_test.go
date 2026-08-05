package middleware

import (
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

func TestAuthRateLimit_Debug(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})

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
		ip := c.ClientIP()
		fmt.Printf("Handler called - ClientIP: %s\n", ip)
		c.String(200, "ok")
	})

	// First 3 requests should succeed
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Forwarded-For", "192.168.1.1")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		fmt.Printf("Request %d: status=%d\n", i+1, w.Code)
	}

	// 4th request should be rate limited
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Forwarded-For", "192.168.1.1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	fmt.Printf("Request 4: status=%d\n", w.Code)

	// Check redis keys
	keys := mr.Keys()
	fmt.Printf("Redis keys: %v\n", keys)
}
