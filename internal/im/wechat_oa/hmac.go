package wechat_oa

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"
)

// SignHMAC builds hex(HMAC-SHA256(secret, timestamp + "\n" + body)).
func SignHMAC(secret, timestamp string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("\n"))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifyHMAC checks Cloud→instance callback signatures.
func VerifyHMAC(
	secret, timestamp string,
	body []byte,
	signature string,
	now time.Time,
	skew time.Duration,
) error {
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid timestamp")
	}
	delta := now.Unix() - ts
	if delta < 0 {
		delta = -delta
	}
	if time.Duration(delta)*time.Second > skew {
		return fmt.Errorf("timestamp skew")
	}
	expected := SignHMAC(secret, timestamp, body)
	if !hmac.Equal([]byte(expected), []byte(signature)) {
		return fmt.Errorf("bad signature")
	}
	return nil
}
