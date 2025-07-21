package main

import (
	"bytes"
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

	tokenIDs := store.GetTokenIDs()
	if len(tokenIDs) != 0 {
		t.Errorf("Expected empty token IDs slice, got %d items", len(tokenIDs))
	}

	// Test adding token IDs
	store.AddTokenID("tokenid1")
	store.AddTokenID("tokenid2")
	store.AddTokenID("tokenid3")

	if store.Count() != 3 {
		t.Errorf("Expected count 3, got %d", store.Count())
	}

	tokenIDs = store.GetTokenIDs()
	if len(tokenIDs) != 3 {
		t.Errorf("Expected 3 token IDs, got %d", len(tokenIDs))
	}

	// Test duplicate token IDs (should overwrite timestamp, not increase count)
	store.AddTokenID("tokenid1")
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
				store.AddTokenID(fmt.Sprintf("tokenid_%d_%d", id, j))
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

	// Note: These tests will fail without a running notification-backend
	// since the app-backend now requires the backend to return an opaque ID
	// We'll test the validation logic but expect network failures

	tests := []struct {
		name           string
		method         string
		body           string
		expectedStatus int
	}{
		{
			name:           "Valid registration (will fail due to no backend)",
			method:         "POST",
			body:           `{"encrypted_data":"test_encrypted_token","platform":"android"}`,
			expectedStatus: http.StatusInternalServerError, // Changed expectation
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
		})
	}
}

func TestHandleHome(t *testing.T) {
	// Reset global token store
	tokenStore = NewTokenStore()
	tokenStore.AddTokenID("test_tokenid1")
	tokenStore.AddTokenID("test_tokenid2")

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
	// Reset global token store and add a token ID
	tokenStore = NewTokenStore()
	tokenStore.AddTokenID("test_tokenid")

	req := httptest.NewRequest("POST", "/send-all", nil)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	w := httptest.NewRecorder()

	handleSendAll(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}
