package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRequireOwnershipOrSystemAdmin_CreatorAllowed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	lookup := CreatorLookup(func(c *gin.Context) (string, error) {
		return "creator-1", nil
	})
	r := gin.New()
	r.Use(func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), types.TenantRoleContextKey, types.TenantRoleAdmin)
		ctx = context.WithValue(ctx, types.UserIDContextKey, "creator-1")
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	r.PUT("/agents/:id",
		RequireOwnershipOrSystemAdmin(lookup, cfgRBAC(true)),
		func(c *gin.Context) { c.Status(http.StatusOK) },
	)
	req := httptest.NewRequest(http.MethodPut, "/agents/a1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
}

func TestRequireOwnershipOrSystemAdmin_PeerAdminRejected(t *testing.T) {
	gin.SetMode(gin.TestMode)
	lookup := CreatorLookup(func(c *gin.Context) (string, error) {
		return "creator-1", nil
	})
	r := gin.New()
	r.Use(func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), types.TenantRoleContextKey, types.TenantRoleAdmin)
		ctx = context.WithValue(ctx, types.UserIDContextKey, "admin-2")
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	r.PUT("/agents/:id",
		RequireOwnershipOrSystemAdmin(lookup, cfgRBAC(true)),
		func(c *gin.Context) { c.Status(http.StatusOK) },
	)
	req := httptest.NewRequest(http.MethodPut, "/agents/a1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusForbidden, w.Code)
}
