package docparser

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
)

func TestPaddleOCRVLReaderSendsBearerAPIKey(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, req *http.Request) {
		gotAuth = req.Header.Get("Authorization")
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(
			`{"errorCode":0,"result":{"layoutParsingResults":[{"markdown":{"text":"hello","images":{}}}]}}`,
		))
	}))
	defer server.Close()

	reader := NewPaddleOCRVLReader(map[string]string{
		"paddleocr_vl_endpoint": server.URL,
		"paddleocr_vl_api_key":  "test-key",
	})
	result, err := reader.Read(context.Background(), &types.ReadRequest{
		FileName:    "demo.png",
		FileType:    "png",
		FileContent: []byte("fake-image"),
	})
	if err != nil {
		t.Fatalf("Read returned error: %v", err)
	}
	if result == nil || result.MarkdownContent != "hello" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if gotAuth != "Bearer test-key" {
		t.Fatalf("Authorization = %q, want %q", gotAuth, "Bearer test-key")
	}
}

func TestPaddleOCRVLReaderOmitsAuthorizationWhenAPIKeyEmpty(t *testing.T) {
	var gotAuth string
	var headerPresent bool
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, req *http.Request) {
		gotAuth = req.Header.Get("Authorization")
		_, headerPresent = req.Header["Authorization"]
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(
			`{"errorCode":0,"result":{"layoutParsingResults":[{"markdown":{"text":"ok","images":{}}}]}}`,
		))
	}))
	defer server.Close()

	reader := NewPaddleOCRVLReader(map[string]string{
		"paddleocr_vl_endpoint": server.URL,
	})
	_, err := reader.Read(context.Background(), &types.ReadRequest{
		FileName:    "demo.png",
		FileType:    "png",
		FileContent: []byte("fake-image"),
	})
	if err != nil {
		t.Fatalf("Read returned error: %v", err)
	}
	if headerPresent || gotAuth != "" {
		t.Fatalf("expected no Authorization header, got present=%v value=%q", headerPresent, gotAuth)
	}
}

func TestPaddleOCRVLReaderSendsDefaultModel(t *testing.T) {
	gotModel := captureLayoutParsingModel(t, map[string]string{
		"paddleocr_vl_endpoint": "placeholder",
	})
	if gotModel != "PaddleOCR-VL" {
		t.Fatalf("model = %q, want %q", gotModel, "PaddleOCR-VL")
	}
}

func TestPaddleOCRVLReaderSendsConfiguredModel(t *testing.T) {
	gotModel := captureLayoutParsingModel(t, map[string]string{
		"paddleocr_vl_endpoint": "placeholder",
		"paddleocr_vl_model":    "PaddleOCR-VL-1.5",
	})
	if gotModel != "PaddleOCR-VL-1.5" {
		t.Fatalf("model = %q, want %q", gotModel, "PaddleOCR-VL-1.5")
	}
}

func captureLayoutParsingModel(t *testing.T, overrides map[string]string) string {
	t.Helper()
	var gotModel string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, req *http.Request) {
		var payload map[string]interface{}
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Errorf("decode payload: %v", err)
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		if model, ok := payload["model"].(string); ok {
			gotModel = model
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(
			`{"errorCode":0,"result":{"layoutParsingResults":[{"markdown":{"text":"ok","images":{}}}]}}`,
		))
	}))
	defer server.Close()

	overrides["paddleocr_vl_endpoint"] = server.URL
	reader := NewPaddleOCRVLReader(overrides)
	_, err := reader.Read(context.Background(), &types.ReadRequest{
		FileName:    "demo.png",
		FileType:    "png",
		FileContent: []byte("fake-image"),
	})
	if err != nil {
		t.Fatalf("Read returned error: %v", err)
	}
	return gotModel
}

