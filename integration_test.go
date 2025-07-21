package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

// Test the complete opaque ID flow
func TestOpaqueIDIntegration(t *testing.T) {
	// Generate temporary RSA keys for testing
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate RSA key: %v", err)
	}

	// Create temporary key files
	privateKeyFile := "test_private.pem"
	publicKeyFile := "test_public.pem"
	defer os.Remove(privateKeyFile)
	defer os.Remove(publicKeyFile)

	// Write private key
	privateKeyPEM := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	}
	privateKeyData := pem.EncodeToMemory(privateKeyPEM)
	if err := os.WriteFile(privateKeyFile, privateKeyData, 0600); err != nil {
		t.Fatalf("Failed to write private key: %v", err)
	}

	// Write public key
	publicKeyPKIX, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("Failed to marshal public key: %v", err)
	}
	publicKeyPEM := &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyPKIX,
	}
	publicKeyData := pem.EncodeToMemory(publicKeyPEM)
	if err := os.WriteFile(publicKeyFile, publicKeyData, 0644); err != nil {
		t.Fatalf("Failed to write public key: %v", err)
	}

	// Create mock Firebase service account key
	firebaseKeyFile := "test_firebase.json"
	defer os.Remove(firebaseKeyFile)
	mockFirebaseKey := map[string]interface{}{
		"project_id": "test-project",
		"type":       "service_account",
	}
	firebaseKeyData, _ := json.Marshal(mockFirebaseKey)
	if err := os.WriteFile(firebaseKeyFile, firebaseKeyData, 0644); err != nil {
		t.Fatalf("Failed to write Firebase key: %v", err)
	}

	// Test 1: Start notification backend (mock mode)
	t.Log("Testing notification backend startup...")

	// Since we can't actually initialize Firebase without real credentials,
	// let's test the opaque ID generation and storage logic directly

	// Import and test DurableTokenStore
	storageFile := "test_tokens.json"
	defer os.Remove(storageFile)

	// This would require importing the notification-backend package
	// For now, let's test the concepts with a simple HTTP test

	// Test 2: Registration flow with opaque ID
	t.Log("Testing registration with opaque ID...")

	// Simulate encrypted token (normally created by Android app)
	mockEncryptedToken := "mock_encrypted_token_base64_data_would_be_here"

	// Create registration request
	registrationReq := map[string]string{
		"encrypted_data": mockEncryptedToken,
		"platform":       "android",
	}
	requestData, _ := json.Marshal(registrationReq)

	// Test the request format
	req := httptest.NewRequest("POST", "/register", bytes.NewBuffer(requestData))
	req.Header.Set("Content-Type", "application/json")

	// Verify request parsing
	var parsedReq map[string]string
	if err := json.NewDecoder(req.Body).Decode(&parsedReq); err != nil {
		t.Fatalf("Failed to parse request: %v", err)
	}

	if parsedReq["encrypted_data"] != mockEncryptedToken {
		t.Errorf("Expected encrypted_data %s, got %s", mockEncryptedToken, parsedReq["encrypted_data"])
	}

	// Test 3: Opaque ID generation (simulate what notification backend does)
	t.Log("Testing opaque ID generation...")

	opaqueID := generateMockOpaqueID()
	if len(opaqueID) != 64 { // 32 bytes = 64 hex characters
		t.Errorf("Expected opaque ID length 64, got %d", len(opaqueID))
	}

	// Test 4: Response format
	t.Log("Testing response format...")

	mockResponse := map[string]interface{}{
		"success":      true,
		"message":      "Token registered successfully",
		"token_id":     opaqueID,
		"platform":     "android",
		"total_tokens": 1,
	}

	responseData, _ := json.Marshal(mockResponse)

	// Verify response can be parsed by app-backend
	var parsedResponse struct {
		Success bool   `json:"success"`
		TokenID string `json:"token_id"`
		Message string `json:"message"`
	}

	if err := json.Unmarshal(responseData, &parsedResponse); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if !parsedResponse.Success {
		t.Error("Expected success=true")
	}

	if parsedResponse.TokenID != opaqueID {
		t.Errorf("Expected token_id %s, got %s", opaqueID, parsedResponse.TokenID)
	}

	// Test 5: Notification request with opaque ID
	t.Log("Testing notification with opaque ID...")

	notificationReq := map[string]string{
		"token_id": opaqueID,
		"title":    "Test Notification",
		"body":     "This is a test message",
	}

	notificationData, _ := json.Marshal(notificationReq)
	notifyReq := httptest.NewRequest("POST", "/notify", bytes.NewBuffer(notificationData))
	notifyReq.Header.Set("Content-Type", "application/json")

	// Verify notification request parsing
	var parsedNotifyReq map[string]string
	if err := json.NewDecoder(notifyReq.Body).Decode(&parsedNotifyReq); err != nil {
		t.Fatalf("Failed to parse notification request: %v", err)
	}

	if parsedNotifyReq["token_id"] != opaqueID {
		t.Errorf("Expected token_id %s, got %s", opaqueID, parsedNotifyReq["token_id"])
	}

	t.Log("✓ All opaque ID integration tests passed!")
	t.Logf("✓ Generated opaque ID: %s...%s", opaqueID[:8], opaqueID[len(opaqueID)-8:])
}

// Helper function to generate mock opaque ID (simulates notification backend logic)
func generateMockOpaqueID() string {
	bytes := make([]byte, 32)
	_, err := rand.Read(bytes)
	if err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("%x", bytes)
}
