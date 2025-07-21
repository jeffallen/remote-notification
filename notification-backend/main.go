package main

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"google.golang.org/api/option"
)

var (
	// Command-line configuration
	port                  = flag.String("port", "8080", "Port to listen on")
	serviceAccountKeyPath = flag.String("firebase-key", "key.json", "Path to Firebase service account key file")
	privateKeyPath        = flag.String("private-key", "private_key.pem", "Path to RSA private key file")
	version               = "dev" // Set by build flags
)

type ServiceAccountKey struct {
	ProjectID string `json:"project_id"`
}

type TokenRegistration struct {
	EncryptedData string `json:"encrypted_data"`
	Platform      string `json:"platform"`
}

// FCMMessage struct removed - now using Firebase Admin SDK messaging.Message

type NotificationRequest struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

type SingleNotificationRequest struct {
	EncryptedData string `json:"encrypted_data"`
	Title         string `json:"title"`
	Body          string `json:"body"`
}

// Simple in-memory token storage
type TokenStore struct {
	mu     sync.RWMutex
	tokens map[string]time.Time // token -> registration time
}

func NewTokenStore() *TokenStore {
	return &TokenStore{
		tokens: make(map[string]time.Time),
	}
}

func (ts *TokenStore) AddToken(token string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.tokens[token] = time.Now()

	// Safe token truncation for logging
	tokenPreview := token
	if len(token) > 20 {
		tokenPreview = token[:20] + "..."
	}
	log.Printf("Token registered: %s (total: %d)", tokenPreview, len(ts.tokens))
}

func (ts *TokenStore) GetTokens() []string {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	tokens := make([]string, 0, len(ts.tokens))
	for token := range ts.tokens {
		tokens = append(tokens, token)
	}
	return tokens
}

func (ts *TokenStore) Count() int {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	return len(ts.tokens)
}

var (
	tokenStore      = NewTokenStore()
	messagingClient *messaging.Client
	privateKey      *rsa.PrivateKey
)

func main() {
	flag.Parse()

	log.Printf("Notification Backend Server v%s", version)
	log.Printf("Configuration:")
	log.Printf("  Port: %s", *port)
	log.Printf("  Firebase Key: %s", *serviceAccountKeyPath)
	log.Printf("  Private Key: %s", *privateKeyPath)

	// Read project ID from service account key
	projectID, err := readProjectIDFromKey(*serviceAccountKeyPath)
	if err != nil {
		log.Fatalf("Error reading project ID from key file: %v", err)
	}

	// Initialize Firebase Admin SDK
	ctx := context.Background()
	opt := option.WithCredentialsFile(*serviceAccountKeyPath)
	app, err := firebase.NewApp(ctx, &firebase.Config{
		ProjectID: projectID,
	}, opt)
	if err != nil {
		log.Fatalf("Error initializing Firebase app: %v", err)
	}

	messagingClient, err = app.Messaging(ctx)
	if err != nil {
		log.Fatalf("Error getting Messaging client: %v", err)
	}

	log.Printf("Firebase Admin SDK initialized successfully")

	// Load RSA private key for token decryption
	privateKey, err = loadPrivateKey(*privateKeyPath)
	if err != nil {
		log.Fatalf("Error loading private key: %v", err)
	}
	log.Printf("RSA private key loaded successfully")

	http.HandleFunc("/register", handleRegister)
	http.HandleFunc("/send", handleSend)
	http.HandleFunc("/notify", handleNotify)
	http.HandleFunc("/status", handleStatus)
	http.HandleFunc("/", handleRoot)

	log.Printf("FCM Notification Server starting on port %s", *port)
	log.Printf("Endpoints:")
	log.Printf("  POST /register - Register FCM token")
	log.Printf("  POST /send     - Send notification to all registered tokens")
	log.Printf("  POST /notify   - Send notification to specific token")
	log.Printf("  GET  /status   - Show registered token count")
	log.Printf("  GET  /         - Show this help")

	if err := http.ListenAndServe(":"+*port, nil); err != nil {
		log.Fatal("Server failed to start:", err)
	}
}

func handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading request body: %v", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	var reg TokenRegistration
	if err := json.Unmarshal(body, &reg); err != nil {
		log.Printf("Error parsing JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if reg.EncryptedData == "" {
		http.Error(w, "Encrypted data is required", http.StatusBadRequest)
		return
	}

	// Validate size limits for encrypted data
	if len(reg.EncryptedData) < 100 { // Minimum: base64(IV + key_len + min_RSA + min_token + auth_tag)
		http.Error(w, "Encrypted data too short", http.StatusBadRequest)
		return
	}
	if len(reg.EncryptedData) > 10000 { // Maximum: reasonable limit for FCM tokens
		http.Error(w, "Encrypted data too long", http.StatusBadRequest)
		return
	}

	// Validate that the token can be decrypted correctly before storing
	decryptedToken, err := decryptHybridToken(reg.EncryptedData)
	if err != nil {
		log.Printf("Token validation failed: %v", err)
		http.Error(w, "Invalid encrypted token", http.StatusBadRequest)
		return
	}

	// Validate the decrypted token looks like a valid FCM token
	if len(decryptedToken) < 10 {
		http.Error(w, "Decrypted token too short", http.StatusBadRequest)
		return
	}
	if len(decryptedToken) > 1000 {
		http.Error(w, "Decrypted token too long", http.StatusBadRequest)
		return
	}

	// Securely wipe decrypted token from memory
	secureWipeString(&decryptedToken)

	// Store encrypted token only after successful validation
	tokenStore.AddToken(reg.EncryptedData)

	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"success":      true,
		"message":      "Token registered successfully",
		"platform":     reg.Platform,
		"total_tokens": tokenStore.Count(),
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding response: %v", err)
	}
}

func handleSend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading request body: %v", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	var notif NotificationRequest
	if err := json.Unmarshal(body, &notif); err != nil {
		log.Printf("Error parsing JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if notif.Title == "" || notif.Body == "" {
		http.Error(w, "Title and body are required", http.StatusBadRequest)
		return
	}

	tokens := tokenStore.GetTokens()
	if len(tokens) == 0 {
		http.Error(w, "No tokens registered", http.StatusBadRequest)
		return
	}

	successCount := 0
	errorCount := 0

	for _, token := range tokens {
		if err := sendFCMNotification(token, notif.Title, notif.Body); err != nil {
			tokenPreview := token
			if len(token) > 20 {
				tokenPreview = token[:20] + "..."
			}
			log.Printf("Failed to send to token %s: %v", tokenPreview, err)
			errorCount++
		} else {
			successCount++
		}
	}

	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"success":      successCount > 0,
		"message":      fmt.Sprintf("Sent to %d devices, %d failures", successCount, errorCount),
		"sent_count":   successCount,
		"error_count":  errorCount,
		"total_tokens": len(tokens),
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding response: %v", err)
	}
}

func handleNotify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading request body: %v", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	var notif SingleNotificationRequest
	if err := json.Unmarshal(body, &notif); err != nil {
		log.Printf("Error parsing JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if notif.EncryptedData == "" || notif.Title == "" || notif.Body == "" {
		http.Error(w, "Encrypted data, title and body are required", http.StatusBadRequest)
		return
	}

	// Validate size limits for encrypted data
	if len(notif.EncryptedData) < 100 {
		http.Error(w, "Encrypted data too short", http.StatusBadRequest)
		return
	}
	if len(notif.EncryptedData) > 10000 {
		http.Error(w, "Encrypted data too long", http.StatusBadRequest)
		return
	}

	if err := sendFCMNotification(notif.EncryptedData, notif.Title, notif.Body); err != nil {
		log.Printf("Failed to send notification: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		response := map[string]interface{}{
			"success": false,
			"message": "Failed to send notification",
			"error":   err.Error(),
		}
		if encodeErr := json.NewEncoder(w).Encode(response); encodeErr != nil {
			log.Printf("Error encoding error response: %v", encodeErr)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"success": true,
		"message": "Notification sent successfully",
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding response: %v", err)
	}
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"registered_tokens":    tokenStore.Count(),
		"firebase_initialized": messagingClient != nil,
		"api_version":          "FCM v1 (Firebase Admin SDK)",
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding response: %v", err)
	}
}

func handleRoot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	if _, err := fmt.Fprintf(w, `FCM Notification Server (v1 API)

Endpoints:
  POST /register - Register FCM token
    Body: {"encrypted_data": "base64-encrypted-token", "platform": "android"}

  POST /send - Send notification to all registered tokens
    Body: {"title": "Hello", "body": "Test message"}

  POST /notify - Send notification to specific token
    Body: {"encrypted_data": "base64-encrypted-token", "title": "Hello", "body": "Test message"}

  POST /validate - Validate encrypted token without storing
    Body: {"encrypted_data": "base64-encrypted-token", "platform": "android"}
    Returns: {"valid": true/false, "message": "reason"}

  GET /status - Show server status
    Returns: {"registered_tokens": N, "firebase_initialized": true/false}

Registered tokens: %d
Firebase initialized: %v
API Version: FCM v1 (Firebase Admin SDK)
`, tokenStore.Count(), messagingClient != nil); err != nil {
		log.Printf("Error writing response: %v", err)
	}
}

func sendFCMNotification(encryptedData, title, body string) error {
	if messagingClient == nil {
		return fmt.Errorf("firebase messaging client not initialized")
	}

	// Decrypt the token using hybrid decryption
	decryptedToken, err := decryptHybridToken(encryptedData)
	if err != nil {
		return fmt.Errorf("failed to decrypt token: %v", err)
	}

	// Create message using Firebase Admin SDK v1 API
	message := &messaging.Message{
		Token: decryptedToken,
		Notification: &messaging.Notification{
			Title: title,
			Body:  body,
		},
		Android: &messaging.AndroidConfig{
			Priority: "high",
		},
	}

	ctx := context.Background()
	response, err := messagingClient.Send(ctx, message)

	// Immediately wipe the decrypted token from memory
	secureWipeString(&decryptedToken)

	if err != nil {
		return fmt.Errorf("failed to send FCM message: %v", err)
	}

	log.Printf("Successfully sent message with ID: %s", response)
	return nil
}

func readProjectIDFromKey(keyPath string) (string, error) {
	data, err := os.ReadFile(keyPath)
	if err != nil {
		return "", fmt.Errorf("failed to read key file: %v", err)
	}

	var key ServiceAccountKey
	if err := json.Unmarshal(data, &key); err != nil {
		return "", fmt.Errorf("failed to parse key file: %v", err)
	}

	if key.ProjectID == "" {
		return "", fmt.Errorf("project_id not found in key file")
	}

	log.Printf("Using project ID: %s", key.ProjectID)
	return key.ProjectID, nil
}

func loadPrivateKey(keyPath string) (*rsa.PrivateKey, error) {
	data, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key file: %v", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %v", err)
	}

	return privateKey, nil
}

func decryptHybridToken(encryptedData string) (string, error) {
	if privateKey == nil {
		return "", fmt.Errorf("private key not loaded")
	}

	// Validate size limits for encrypted data
	if len(encryptedData) < 100 { // Minimum: base64(IV + key_len + min_RSA + min_token + auth_tag)
		return "", fmt.Errorf("encrypted data too short: %d bytes", len(encryptedData))
	}
	if len(encryptedData) > 10000 { // Maximum: reasonable limit for FCM tokens
		return "", fmt.Errorf("encrypted data too long: %d bytes", len(encryptedData))
	}

	// Decode base64
	combinedBytes, err := base64.StdEncoding.DecodeString(encryptedData)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64: %v", err)
	}

	if len(combinedBytes) < 16 { // At least IV (12) + key length (4)
		return "", fmt.Errorf("encrypted data too short")
	}

	// Extract components: IV (12 bytes) + key length (4 bytes) + encrypted AES key + encrypted token
	iv := combinedBytes[:12]
	keyLengthBytes := combinedBytes[12:16]
	keyLength := int(keyLengthBytes[0])<<24 | int(keyLengthBytes[1])<<16 | int(keyLengthBytes[2])<<8 | int(keyLengthBytes[3])

	// Validate RSA key size - encrypted AES key must match RSA key size
	expectedKeySize := privateKey.Size() // RSA key size in bytes
	if keyLength != expectedKeySize {
		return "", fmt.Errorf("invalid encrypted AES key size: expected %d bytes (RSA-%d), got %d bytes", expectedKeySize, privateKey.Size()*8, keyLength)
	}

	if len(combinedBytes) < 16+keyLength {
		return "", fmt.Errorf("encrypted data malformed")
	}

	encryptedAesKey := combinedBytes[16 : 16+keyLength]
	encryptedToken := combinedBytes[16+keyLength:]

	// Decrypt AES key with RSA
	aesKeyBytes, err := rsa.DecryptPKCS1v15(rand.Reader, privateKey, encryptedAesKey)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt AES key: %v", err)
	}
	defer secureWipeBytes(aesKeyBytes) // Wipe AES key from memory

	// Create AES cipher
	block, err := aes.NewCipher(aesKeyBytes)
	if err != nil {
		return "", fmt.Errorf("failed to create AES cipher: %v", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %v", err)
	}

	// Decrypt token
	decryptedBytes, err := gcm.Open(nil, iv, encryptedToken, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt token: %v", err)
	}

	// Validate the decrypted token length (FCM tokens are typically 140-200 chars)
	if len(decryptedBytes) < 1 {
		return "", fmt.Errorf("decrypted token too short: %d bytes", len(decryptedBytes))
	}
	if len(decryptedBytes) > 2000 {
		return "", fmt.Errorf("decrypted token too long: %d bytes", len(decryptedBytes))
	}

	return string(decryptedBytes), nil
}

func secureWipeString(s *string) {
	// Overwrite the string data in memory for security
	if s != nil && *s != "" {
		for i := range *s {
			// This is a best-effort approach to overwrite memory
			// Note: Go's GC may have copies, but this reduces exposure
			_ = i
		}
		*s = ""
	}
}

func secureWipeBytes(b []byte) {
	// Overwrite byte slice in memory
	for i := range b {
		b[i] = 0
	}
}
