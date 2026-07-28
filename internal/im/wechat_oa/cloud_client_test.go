package wechat_oa

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHTTPCloudClient_CreatePreAuthAndSendText(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/oa/preauth", func(writer http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost {
			http.Error(writer, "method", http.StatusMethodNotAllowed)
			return
		}
		if req.Header.Get("X-APPID") == "" || req.Header.Get("X-Signature") == "" {
			http.Error(writer, "unsigned", http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(writer).Encode(PreAuthResponse{
			PreAuthID:      "pa_1",
			QRCodeURL:      "https://example.com/qr.png",
			ExpiresAt:      time.Now().Add(30 * time.Minute),
			CallbackSecret: "cb_secret",
		})
	})
	mux.HandleFunc("/api/v1/oa/message/send", func(writer http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost {
			http.Error(writer, "method", http.StatusMethodNotAllowed)
			return
		}
		var payload map[string]any
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
		if payload["authorizer_appid"] != "wxapp" || payload["touser"] != "o1" {
			http.Error(writer, "bad payload", http.StatusBadRequest)
			return
		}
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte(`{}`))
	})
	mux.HandleFunc("/api/v1/oa/preauth/pa_1", func(writer http.ResponseWriter, req *http.Request) {
		_ = json.NewEncoder(writer).Encode(PreAuthStatus{Status: "wait"})
	})
	mux.HandleFunc("/api/v1/oa/bindings/wxapp/unbind", func(writer http.ResponseWriter, req *http.Request) {
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte(`{}`))
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	client := NewHTTPCloudClient(server.URL, "app", "secret", server.Client())
	ctx := context.Background()

	preauth, err := client.CreatePreAuth(ctx, PreAuthRequest{
		InstanceBaseURL: "https://inst.example",
		TenantID:        1,
		AgentID:         "ag1",
		State:           "st1",
	})
	if err != nil {
		t.Fatalf("CreatePreAuth: %v", err)
	}
	if preauth.PreAuthID != "pa_1" || preauth.CallbackSecret != "cb_secret" {
		t.Fatalf("unexpected preauth: %+v", preauth)
	}

	status, err := client.GetPreAuth(ctx, "pa_1")
	if err != nil || status.Status != "wait" {
		t.Fatalf("GetPreAuth: err=%v status=%+v", err, status)
	}
	if err := client.SendText(ctx, "wxapp", "o1", "hi"); err != nil {
		t.Fatalf("SendText: %v", err)
	}
	if err := client.Unbind(ctx, "wxapp"); err != nil {
		t.Fatalf("Unbind: %v", err)
	}
}

func TestHTTPCloudClient_CreatePreAuth_RequiresCallbackSecret(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/oa/preauth", func(writer http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(writer).Encode(PreAuthResponse{
			PreAuthID: "pa_1",
			QRCodeURL: "https://example.com/qr.png",
		})
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	client := NewHTTPCloudClient(server.URL, "app", "secret", server.Client())
	_, err := client.CreatePreAuth(context.Background(), PreAuthRequest{
		InstanceBaseURL: "https://inst.example",
		TenantID:        1,
		AgentID:         "ag1",
		State:           "st1",
	})
	if err == nil || !strings.Contains(err.Error(), "callback_secret") {
		t.Fatalf("expected callback_secret error, got %v", err)
	}
}
