package wechat_oa

import (
	"strconv"
	"testing"
	"time"
)

func TestVerifyHMAC_AcceptsValid(t *testing.T) {
	secret := "sekrit"
	body := []byte(`{"msg_id":"1"}`)
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	sig := SignHMAC(secret, ts, body)
	if err := VerifyHMAC(secret, ts, body, sig, time.Now(), 5*time.Minute); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyHMAC_RejectsTampered(t *testing.T) {
	secret := "sekrit"
	body := []byte(`{"msg_id":"1"}`)
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	sig := SignHMAC(secret, ts, body)
	if err := VerifyHMAC(
		secret, ts, []byte(`{"msg_id":"2"}`), sig, time.Now(), 5*time.Minute,
	); err == nil {
		t.Fatal("expected error")
	}
}
