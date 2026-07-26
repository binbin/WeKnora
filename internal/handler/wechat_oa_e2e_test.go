package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/im/wechat_oa"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
)

func TestBindingComplete_ValidHMAC_RequiresIMService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newMemPreauthRepo()
	secret := "sekrit"
	_ = repo.Create(context.Background(), &types.WeChatOAPreauth{
		ID: "pa2", TenantID: 1, AgentID: "ag", State: "st2", Status: "wait",
		CallbackSecret: secret, ExpiresAt: time.Now().Add(time.Hour),
	})
	h := &WeChatOAHandler{preauthRepo: repo}
	bodyObj := map[string]any{
		"state": "st2", "authorizer_appid": "wx1",
		"cloud_binding_id": "bnd", "instance_callback_secret": "ics",
	}
	raw, _ := json.Marshal(bodyObj)
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	sig := wechat_oa.SignHMAC(secret, ts, raw)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-WeKnora-OA-Timestamp", ts)
	req.Header.Set("X-WeKnora-OA-Signature", sig)
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = req
	h.BindingComplete(ctx)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
}
