package wechat_oa

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/im"
	"github.com/gin-gonic/gin"
)

type fakeCloud struct {
	sent string
}

func (fake *fakeCloud) CreatePreAuth(
	context.Context, PreAuthRequest,
) (*PreAuthResponse, error) {
	return nil, nil
}

func (fake *fakeCloud) GetPreAuth(
	context.Context, string,
) (*PreAuthStatus, error) {
	return nil, nil
}

func (fake *fakeCloud) Unbind(context.Context, string) error { return nil }

func (fake *fakeCloud) SendText(
	_ context.Context, _, _, text string,
) error {
	fake.sent = text
	return nil
}

func TestAdapter_ParseCallback_Text(t *testing.T) {
	gin.SetMode(gin.TestMode)
	secret := "sekrit"
	event := RelayEvent{
		RelayEventID: "r1",
		MsgID:        "m1",
		FromUser:     "openid1",
		MsgType:      "text",
		Content:      "hello",
	}
	body, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	sig := SignHMAC(secret, ts, body)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/callback", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-WeKnora-OA-Timestamp", ts)
	req.Header.Set("X-WeKnora-OA-Signature", sig)
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = req

	adapter := NewAdapter("wxapp", secret, &fakeCloud{})
	if err := adapter.VerifyCallback(ctx); err != nil {
		t.Fatalf("VerifyCallback: %v", err)
	}
	msg, err := adapter.ParseCallback(ctx)
	if err != nil {
		t.Fatalf("ParseCallback: %v", err)
	}
	if msg == nil || msg.UserID != "openid1" || msg.Content != "hello" || msg.MessageID != "m1" {
		t.Fatalf("unexpected message: %+v", msg)
	}
	if msg.Platform != im.PlatformWeChatOA {
		t.Fatalf("platform=%s", msg.Platform)
	}
}

func TestAdapter_SendReply_CallsCloud(t *testing.T) {
	fake := &fakeCloud{}
	adapter := NewAdapter("wxapp", "secret", fake)
	err := adapter.SendReply(
		context.Background(),
		&im.IncomingMessage{UserID: "o1"},
		&im.ReplyMessage{Content: "hi"},
	)
	if err != nil || fake.sent != "hi" {
		t.Fatalf("err=%v sent=%q", err, fake.sent)
	}
}
