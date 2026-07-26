package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/im/wechat_oa"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
)

type memPreauthRepo struct {
	mu   sync.Mutex
	byID map[string]*types.WeChatOAPreauth
}

func newMemPreauthRepo() *memPreauthRepo {
	return &memPreauthRepo{byID: map[string]*types.WeChatOAPreauth{}}
}

func (m *memPreauthRepo) Create(_ context.Context, row *types.WeChatOAPreauth) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if row.ID == "" {
		row.ID = "pa_test"
	}
	cp := *row
	m.byID[row.ID] = &cp
	return nil
}

func (m *memPreauthRepo) GetByID(_ context.Context, id string) (*types.WeChatOAPreauth, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	row := m.byID[id]
	if row == nil {
		return nil, nil
	}
	cp := *row
	return &cp, nil
}

func (m *memPreauthRepo) GetByState(_ context.Context, state string) (*types.WeChatOAPreauth, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, row := range m.byID {
		if row.State == state {
			cp := *row
			return &cp, nil
		}
	}
	return nil, nil
}

func (m *memPreauthRepo) Update(_ context.Context, row *types.WeChatOAPreauth) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *row
	m.byID[row.ID] = &cp
	return nil
}

func TestBindingComplete_RejectsBadHMAC(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newMemPreauthRepo()
	_ = repo.Create(context.Background(), &types.WeChatOAPreauth{
		ID:             "pa1",
		TenantID:       1,
		AgentID:        "ag1",
		State:          "st1",
		Status:         "wait",
		CallbackSecret: "sekrit",
		ExpiresAt:      time.Now().Add(time.Hour),
	})
	h := &WeChatOAHandler{preauthRepo: repo}

	bodyObj := map[string]any{
		"state":                    "st1",
		"authorizer_appid":         "wx1",
		"cloud_binding_id":         "bnd1",
		"instance_callback_secret": "ics1",
	}
	raw, _ := json.Marshal(bodyObj)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/bind", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-WeKnora-OA-Timestamp", strconv.FormatInt(time.Now().Unix(), 10))
	req.Header.Set("X-WeKnora-OA-Signature", "deadbeef")
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = req
	h.BindingComplete(ctx)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestVerifyHMAC_UsedByBindingComplete(t *testing.T) {
	secret := "sekrit"
	body := []byte(`{"state":"st1"}`)
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	sig := wechat_oa.SignHMAC(secret, ts, body)
	if err := wechat_oa.VerifyHMAC(secret, ts, body, sig, time.Now(), 5*time.Minute); err != nil {
		t.Fatal(err)
	}
}
