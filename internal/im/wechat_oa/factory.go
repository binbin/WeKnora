package wechat_oa

import (
	"context"
	"fmt"

	"github.com/Tencent/WeKnora/internal/im"
)

// CloudClientFactory builds a tenant-scoped Cloud OA client.
type CloudClientFactory func(
	ctx context.Context,
	tenantID uint64,
) (CloudClient, error)

// NewFactory returns an im.AdapterFactory for wechat_oa (cloud_relay, no long-poll).
func NewFactory(cloudFactory CloudClientFactory) im.AdapterFactory {
	return func(
		factoryCtx context.Context,
		channel *im.IMChannel,
		_ func(context.Context, *im.IncomingMessage) error,
	) (im.Adapter, context.CancelFunc, error) {
		creds, err := im.ParseCredentials(channel.Credentials)
		if err != nil {
			return nil, nil, fmt.Errorf("parse wechat_oa credentials: %w", err)
		}
		authorizerAppID := im.GetString(creds, "authorizer_appid")
		callbackSecret := im.GetString(creds, "instance_callback_secret")
		if authorizerAppID == "" || callbackSecret == "" {
			return nil, nil, fmt.Errorf(
				"wechat_oa credentials require authorizer_appid and instance_callback_secret",
			)
		}
		if cloudFactory == nil {
			return nil, nil, fmt.Errorf("wechat_oa: cloud client factory not configured")
		}
		cloud, err := cloudFactory(factoryCtx, channel.TenantID)
		if err != nil {
			return nil, nil, fmt.Errorf("wechat_oa cloud client: %w", err)
		}
		adapter := NewAdapter(authorizerAppID, callbackSecret, cloud)
		return adapter, func() {}, nil
	}
}
