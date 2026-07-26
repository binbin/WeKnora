package wechat_oa

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/Tencent/WeKnora/internal/im"
	"github.com/gin-gonic/gin"
)

const hmacSkew = 5 * time.Minute

// Adapter implements im.Adapter for WeChat Official Account via Cloud relay.
type Adapter struct {
	authorizerAppID string
	callbackSecret  string
	cloud           CloudClient
}

// NewAdapter creates a wechat_oa adapter.
func NewAdapter(
	authorizerAppID, callbackSecret string,
	cloud CloudClient,
) *Adapter {
	return &Adapter{
		authorizerAppID: authorizerAppID,
		callbackSecret:  callbackSecret,
		cloud:           cloud,
	}
}

func (adapter *Adapter) Platform() im.Platform {
	return im.PlatformWeChatOA
}

func (adapter *Adapter) HandleURLVerification(c *gin.Context) bool {
	return false
}

func (adapter *Adapter) VerifyCallback(c *gin.Context) error {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return fmt.Errorf("read body: %w", err)
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	timestamp := c.GetHeader("X-WeKnora-OA-Timestamp")
	signature := c.GetHeader("X-WeKnora-OA-Signature")
	return VerifyHMAC(
		adapter.callbackSecret,
		timestamp,
		body,
		signature,
		time.Now(),
		hmacSkew,
	)
}

func (adapter *Adapter) ParseCallback(c *gin.Context) (*im.IncomingMessage, error) {
	var event RelayEvent
	if err := c.ShouldBindJSON(&event); err != nil {
		return nil, err
	}
	if event.MsgType != "text" {
		// P0: ignore non-text (ACK only).
		return nil, nil
	}
	// MessageID feeds IM Service dedup (Redis/local, ~5m TTL).
	messageID := event.MsgID
	if messageID == "" {
		messageID = event.RelayEventID
	}
	return &im.IncomingMessage{
		Platform:    im.PlatformWeChatOA,
		MessageType: im.MessageTypeText,
		UserID:      event.FromUser,
		Content:     event.Content,
		MessageID:   messageID,
		ChatType:    im.ChatTypeDirect,
	}, nil
}

func (adapter *Adapter) SendReply(
	ctx context.Context,
	incoming *im.IncomingMessage,
	reply *im.ReplyMessage,
) error {
	if adapter.cloud == nil {
		return fmt.Errorf("wechat_oa: cloud client not configured")
	}
	return adapter.cloud.SendText(
		ctx,
		adapter.authorizerAppID,
		incoming.UserID,
		reply.Content,
	)
}

