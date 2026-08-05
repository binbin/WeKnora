package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestSecurityHeaders_DefaultValues(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(SecurityHeaders())
	r.GET("/test", func(c *gin.Context) {
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "SAMEORIGIN", w.Header().Get("X-Frame-Options"))
	assert.Equal(t, "1; mode=block", w.Header().Get("X-XSS-Protection"))
	assert.Equal(t, "strict-origin-when-cross-origin", w.Header().Get("Referrer-Policy"))
	assert.Equal(t, "camera=(), microphone=(), geolocation=(), payment=()", w.Header().Get("Permissions-Policy"))
	// HSTS should not be set by default
	assert.Equal(t, "", w.Header().Get("Strict-Transport-Security"))
}

func TestSecurityHeaders_CustomValues(t *testing.T) {
	t.Setenv("X_FRAME_OPTIONS", "DENY")
	t.Setenv("REFERRER_POLICY", "no-referrer")
	t.Setenv("PERMISSIONS_POLICY", "camera=(self)")
	t.Setenv("HSTS_MAX_AGE", "31536000")
	t.Setenv("HSTS_INCLUDE_SUBDOMAINS", "true")

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(SecurityHeaders())
	r.GET("/test", func(c *gin.Context) {
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
	assert.Equal(t, "no-referrer", w.Header().Get("Referrer-Policy"))
	assert.Equal(t, "camera=(self)", w.Header().Get("Permissions-Policy"))
	assert.Equal(t, "max-age=31536000; includeSubDomains", w.Header().Get("Strict-Transport-Security"))
}

func TestEmbedSecurityHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(embedSecurityHeaders())
	r.GET("/embed/test", func(c *gin.Context) {
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/embed/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "1; mode=block", w.Header().Get("X-XSS-Protection"))
	assert.Equal(t, "strict-origin-when-cross-origin", w.Header().Get("Referrer-Policy"))
	// X-Frame-Options should NOT be set for embed pages
	assert.Equal(t, "", w.Header().Get("X-Frame-Options"))
}
