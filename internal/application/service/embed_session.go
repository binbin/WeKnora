package service

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/redis/go-redis/v9"
)

const (
	embedSessionTokenPrefix = "ems_"
	embedSessionRedisPrefix = "embed:session:"
	embedSessionTTL         = 30 * time.Minute
)

var ErrEmbedSessionUnavailable = errors.New("embed session tokens unavailable")

func generateEmbedSessionToken() (string, error) {
	buf := make([]byte, embedTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return embedSessionTokenPrefix + base64.RawURLEncoding.EncodeToString(buf), nil
}

// IssueSessionToken mints a short-lived session token bound to channelID.
func (s *embedChannelService) IssueSessionToken(ctx context.Context, channelID string) (string, int, error) {
	if s.redis == nil {
		return "", 0, ErrEmbedSessionUnavailable
	}
	channelID = strings.TrimSpace(channelID)
	if channelID == "" {
		return "", 0, ErrEmbedTokenInvalid
	}
	token, err := generateEmbedSessionToken()
	if err != nil {
		return "", 0, err
	}
	key := embedSessionRedisPrefix + token
	if err := s.redis.Set(ctx, key, channelID, embedSessionTTL).Err(); err != nil {
		return "", 0, err
	}
	return token, int(embedSessionTTL.Seconds()), nil
}

// ResolveSessionToken returns the channel ID stored for a session token.
func (s *embedChannelService) ResolveSessionToken(ctx context.Context, token string) (string, error) {
	token = strings.TrimSpace(token)
	if !strings.HasPrefix(token, embedSessionTokenPrefix) {
		return "", ErrEmbedTokenInvalid
	}
	if s.redis == nil {
		return "", ErrEmbedSessionUnavailable
	}
	channelID, err := s.redis.Get(ctx, embedSessionRedisPrefix+token).Result()
	if err == redis.Nil {
		return "", ErrEmbedTokenInvalid
	}
	if err != nil {
		return "", err
	}
	channelID = strings.TrimSpace(channelID)
	if channelID == "" {
		return "", ErrEmbedTokenInvalid
	}
	return channelID, nil
}

// LookupEnabledChannel loads an embed channel and verifies it is enabled.
// It resolves against embed_channels first; on a miss it falls back to
// guest_link_channels (mapped via AsEmbedChannel), since both surfaces share
// the same /api/v1/embed/{id}/... routes and EmbedAuth. See the architecture
// note in the guest-link-vs-web-embed design doc.
func (s *embedChannelService) LookupEnabledChannel(ctx context.Context, channelID string) (*types.EmbedChannel, error) {
	channelID = strings.TrimSpace(channelID)
	if channelID == "" {
		return nil, ErrEmbedTokenInvalid
	}
	ch, err := s.repo.GetByID(ctx, channelID)
	if err != nil {
		return nil, err
	}
	if ch != nil {
		if !ch.Enabled {
			return nil, ErrEmbedChannelDisabled
		}
		return ch, nil
	}
	if s.guestLinkRepo == nil {
		return nil, ErrEmbedTokenInvalid
	}
	gl, err := s.guestLinkRepo.GetByID(ctx, channelID)
	if err != nil || gl == nil {
		return nil, ErrEmbedTokenInvalid
	}
	if !gl.Enabled {
		return nil, ErrEmbedChannelDisabled
	}
	return gl.AsEmbedChannel(), nil
}

// IsEmbedSessionToken reports whether token is a session token (ems_ prefix).
func IsEmbedSessionToken(token string) bool {
	return strings.HasPrefix(strings.TrimSpace(token), embedSessionTokenPrefix)
}

// SignEmbedSessionHandle binds a chat session id to its embed channel with an
// HMAC keyed by the channel's (server-only) publish token. The handle is handed
// to the widget at session-creation time and must be presented on every history
// load / chat call. Because the session id travels in the request path (and can
// land in access logs), this signature — sent in a header, never logged — is the
// real authorization secret: a leaked session id is useless without it. Rotating
// the channel token invalidates outstanding handles, which is acceptable.
// An empty key yields an empty handle rather than an HMAC over "": every
// channel surface must supply its own secret, and failing closed here keeps a
// silently unkeyed channel from producing forgeable handles.
func SignEmbedSessionHandle(ch *types.EmbedChannel, sessionID string) string {
	if ch == nil || strings.TrimSpace(sessionID) == "" {
		return ""
	}
	if ch.PublishToken == "" {
		return ""
	}
	mac := hmac.New(sha256.New, []byte(ch.PublishToken))
	mac.Write([]byte(ch.ID + "|" + sessionID))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// VerifyEmbedSessionHandle reports whether sig is a valid handle for sessionID
// on channel ch, using a constant-time comparison.
func VerifyEmbedSessionHandle(ch *types.EmbedChannel, sessionID, sig string) bool {
	sig = strings.TrimSpace(sig)
	if sig == "" {
		return false
	}
	expected := SignEmbedSessionHandle(ch, sessionID)
	if expected == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(expected), []byte(sig)) == 1
}

// IssuePreviewSession mints a short-lived session token for management UI preview.
func (s *embedChannelService) IssuePreviewSession(
	ctx context.Context, tenantID uint64, channelID string,
) (string, int, error) {
	ch, err := s.getOwned(ctx, tenantID, channelID)
	if err != nil {
		return "", 0, err
	}
	if !ch.Enabled {
		return "", 0, ErrEmbedChannelDisabled
	}
	return s.IssueSessionToken(ctx, ch.ID)
}
