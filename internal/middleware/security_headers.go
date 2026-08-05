package middleware

import (
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

// SecurityHeaders returns a middleware that sets common security response headers.
// These headers protect against common web vulnerabilities:
//   - X-Content-Type-Options: Prevents MIME type sniffing
//   - X-Frame-Options: Prevents clickjacking (configurable via X_FRAME_OPTIONS env)
//   - X-XSS-Protection: Enables browser XSS filtering (legacy, but still useful)
//   - Referrer-Policy: Controls referrer information leakage
//   - Permissions-Policy: Restricts browser feature access
//   - Strict-Transport-Security: Enforces HTTPS (only when HSTS_MAX_AGE is set)
//
// Headers can be customized via environment variables:
//   - X_FRAME_OPTIONS: Default "SAMEORIGIN". Set to "DENY" or "ALLOW-FROM uri"
//   - REFERRER_POLICY: Default "strict-origin-when-cross-origin"
//   - HSTS_MAX_AGE: Default empty (disabled). Set to e.g. "31536000" for 1 year
//   - HSTS_INCLUDE_SUBDOMAINS: Default "false". Set to "true" to include subdomains
//   - PERMISSIONS_POLICY: Default restricts camera, microphone, geolocation
func SecurityHeaders() gin.HandlerFunc {
	// Pre-compute header values once at startup
	xFrameOptions := envOrDefault("X_FRAME_OPTIONS", "SAMEORIGIN")
	referrerPolicy := envOrDefault("REFERRER_POLICY", "strict-origin-when-cross-origin")
	permissionsPolicy := envOrDefault("PERMISSIONS_POLICY",
		"camera=(), microphone=(), geolocation=(), payment=()")

	// HSTS is opt-in: only set when explicitly configured
	hstsMaxAge := strings.TrimSpace(os.Getenv("HSTS_MAX_AGE"))
	hstsIncludeSubdomains := strings.EqualFold(
		strings.TrimSpace(os.Getenv("HSTS_INCLUDE_SUBDOMAINS")), "true")

	var hstsValue string
	if hstsMaxAge != "" {
		hstsValue = "max-age=" + hstsMaxAge
		if hstsIncludeSubdomains {
			hstsValue += "; includeSubDomains"
		}
		// Add preload directive only when explicitly requested
		if strings.EqualFold(strings.TrimSpace(os.Getenv("HSTS_PRELOAD")), "true") {
			hstsValue += "; preload"
		}
	}

	return func(c *gin.Context) {
		// Core security headers - always set
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", xFrameOptions)
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Referrer-Policy", referrerPolicy)
		c.Header("Permissions-Policy", permissionsPolicy)

		// HSTS - only when explicitly configured (requires HTTPS)
		if hstsValue != "" {
			c.Header("Strict-Transport-Security", hstsValue)
		}

		c.Next()
	}
}

// embedSecurityHeaders returns a middleware that sets security headers for embed pages.
// Embed pages intentionally omit X-Frame-Options to allow iframe embedding,
// but still need other security protections.
func embedSecurityHeaders() gin.HandlerFunc {
	referrerPolicy := envOrDefault("REFERRER_POLICY", "strict-origin-when-cross-origin")
	permissionsPolicy := envOrDefault("PERMISSIONS_POLICY",
		"camera=(), microphone=(), geolocation=(), payment=()")

	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Referrer-Policy", referrerPolicy)
		c.Header("Permissions-Policy", permissionsPolicy)
		// Note: X-Frame-Options intentionally omitted for embed pages
		c.Next()
	}
}

func envOrDefault(key, defaultVal string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return defaultVal
}
