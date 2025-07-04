package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestTokenStore(t *testing.T) {
	store := NewTokenStore()

	// Test initial state
	if store.Count() != 0 {
		t.Errorf("Expected initial count 0, got %d", store.Count())
	}

	tokens := store.GetTokens()
	if len(tokens) != 0 {
		t.Errorf("Expected empty tokens slice, got %d items", len(tokens))
	}

	// Test adding tokens
	store.AddToken("token1")
	store.AddToken("token2")
	store.AddToken("token3")

	if store.Count() != 3 {
		t.Errorf("Expected count 3, got %d", store.Count())
	}

	tokens = store.GetTokens()
	if len(tokens) != 3 {
		t.Errorf("Expected 3 tokens, got %d", len(tokens))
	}

	// Test duplicate tokens (should overwrite timestamp, not increase count)
	store.AddToken("token1")
	if store.Count() != 3 {
		t.Errorf("Expected count 3 after adding duplicate (map overwrites), got %d", store.Count())
	}
}

func TestTokenStoreConcurrency(t *testing.T) {
	store := NewTokenStore()

	// Test concurrent access
	done := make(chan bool)
	numGoroutines := 10
	tokensPerGoroutine := 100

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			for j := 0; j < tokensPerGoroutine; j++ {
				store.AddToken(fmt.Sprintf("token_%d_%d", id, j))
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		select {
		case <-done:
			// Good
		case <-time.After(5 * time.Second):
			t.Fatal("Test timed out")
		}
	}

	expectedCount := numGoroutines * tokensPerGoroutine
	if store.Count() != expectedCount {
		t.Errorf("Expected count %d, got %d", expectedCount, store.Count())
	}
}

func TestHandleRegister(t *testing.T) {
	// Reset global token store
	tokenStore = NewTokenStore()

	// Note: This test allows network calls to fail (which is expected
	// since notification-backend is not running), but the handler
	// should still register the token successfully

	tests := []struct {
		name           string
		method         string
		body           string
		expectedStatus int
	}{
		{
			name:           "Valid registration",
			method:         "POST",
			body:           `{"encrypted_data":"test_encrypted_token","platform":"android"}`,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Invalid method",
			method:         "GET",
			body:           `{"encrypted_data":"test_encrypted_token","platform":"android"}`,
			expectedStatus: http.StatusMethodNotAllowed,
		},
		{
			name:           "Invalid JSON",
			method:         "POST",
			body:           `{invalid json}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Missing encrypted_data",
			method:         "POST",
			body:           `{"platform":"android"}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Empty encrypted_data",
			method:         "POST",
			body:           `{"encrypted_data":"","platform":"android"}`,
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/register", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handleRegister(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectedStatus == http.StatusOK {
				var response map[string]interface{}
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Errorf("Failed to parse response JSON: %v", err)
				}

				if success, ok := response["success"].(bool); !ok || !success {
					t.Errorf("Expected success=true in response, got %v", response["success"])
				}

				if totalTokens, ok := response["total_tokens"].(float64); !ok || totalTokens < 1 {
					t.Errorf("Expected total_tokens >= 1, got %v", response["total_tokens"])
				}
			}
		})
	}
}

func TestHandleHome(t *testing.T) {
	// Reset global token store
	tokenStore = NewTokenStore()
	tokenStore.AddToken("test_token1")
	tokenStore.AddToken("test_token2")

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	handleHome(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "text/html; charset=utf-8" {
		t.Errorf("Expected Content-Type 'text/html; charset=utf-8', got %q", contentType)
	}

	body := w.Body.String()
	if !strings.Contains(body, "App Backend") {
		t.Error("Expected response to contain 'App Backend' title")
	}

	// Should show token count
	if !strings.Contains(body, "2") {
		t.Error("Expected response to show token count of 2")
	}
}

func TestHandleSendAllInvalidMethod(t *testing.T) {
	req := httptest.NewRequest("GET", "/send-all", nil)
	w := httptest.NewRecorder()

	handleSendAll(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

func TestHandleSendAllNoTokens(t *testing.T) {
	// Reset global token store to empty
	tokenStore = NewTokenStore()

	req := httptest.NewRequest("POST", "/send-all", nil)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Form = map[string][]string{"message": {"test message"}}

	w := httptest.NewRecorder()

	handleSendAll(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestHandleSendAllNoMessage(t *testing.T) {
	// Reset global token store and add a token
	tokenStore = NewTokenStore()
	tokenStore.AddToken("test_token")

	req := httptest.NewRequest("POST", "/send-all", nil)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	w := httptest.NewRecorder()

	handleSendAll(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}
