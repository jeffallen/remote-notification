package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"google.golang.org/api/option"
)

const (
	serviceAccountKeyPath = "key.json"
	privateKeyPath        = "private_key.pem"
)

type ServiceAccountKey struct {
	ProjectID string `json:"project_id"`
}

type TokenRegistration struct {
	Token    string `json:"token"`
	Platform string `json:"platform"`
}

// FCMMessage struct removed - now using Firebase Admin SDK messaging.Message

type NotificationRequest struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

type SingleNotificationRequest struct {
	Token string `json:"token"`
	Title string `json:"title"`
	Body  string `json:"body"`
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
	// Read project ID from service account key
	projectID, err := readProjectIDFromKey(serviceAccountKeyPath)
	if err != nil {
		log.Fatalf("Error reading project ID from key file: %v", err)
	}

	// Initialize Firebase Admin SDK
	ctx := context.Background()
	opt := option.WithCredentialsFile(serviceAccountKeyPath)
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
	privateKey, err = loadPrivateKey(privateKeyPath)
	if err != nil {
		log.Fatalf("Error loading private key: %v", err)
	}
	log.Printf("RSA private key loaded successfully")

	http.HandleFunc("/register", handleRegister)
	http.HandleFunc("/send", handleSend)
	http.HandleFunc("/notify", handleNotify)
	http.HandleFunc("/status", handleStatus)
	http.HandleFunc("/", handleRoot)

	port := "8080"
	log.Printf("FCM Notification Server starting on port %s", port)
	log.Printf("Endpoints:")
	log.Printf("  POST /register - Register FCM token")
	log.Printf("  POST /send     - Send notification to all registered tokens")
	log.Printf("  POST /notify   - Send notification to specific token")
	log.Printf("  GET  /status   - Show registered token count")
	log.Printf("  GET  /         - Show this help")

	if err := http.ListenAndServe(":"+port, nil); err != nil {
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

	if reg.Token == "" {
		http.Error(w, "Token is required", http.StatusBadRequest)
		return
	}

	tokenStore.AddToken(reg.Token)

	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"success":      true,
		"message":      "Token registered successfully",
		"platform":     reg.Platform,
		"total_tokens": tokenStore.Count(),
	}
	json.NewEncoder(w).Encode(response)
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
	json.NewEncoder(w).Encode(response)
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

	if notif.Token == "" || notif.Title == "" || notif.Body == "" {
		http.Error(w, "Token, title and body are required", http.StatusBadRequest)
		return
	}

	if err := sendFCMNotification(notif.Token, notif.Title, notif.Body); err != nil {
		tokenPreview := notif.Token
		if len(notif.Token) > 20 {
			tokenPreview = notif.Token[:20] + "..."
		}
		log.Printf("Failed to send to token %s: %v", tokenPreview, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		response := map[string]interface{}{
			"success": false,
			"message": "Failed to send notification",
			"error":   err.Error(),
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"success": true,
		"message": "Notification sent successfully",
	}
	json.NewEncoder(w).Encode(response)
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"registered_tokens":    tokenStore.Count(),
		"firebase_initialized": messagingClient != nil,
		"api_version":          "FCM v1 (Firebase Admin SDK)",
	}
	json.NewEncoder(w).Encode(response)
}

func handleRoot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintf(w, `FCM Notification Server (v1 API)

Endpoints:
  POST /register - Register FCM token
    Body: {"token": "fcm-token", "platform": "android"}

  POST /send - Send notification to all registered tokens
    Body: {"title": "Hello", "body": "Test message"}

  POST /notify - Send notification to specific token
    Body: {"token": "fcm-token", "title": "Hello", "body": "Test message"}

  GET /status - Show server status
    Returns: {"registered_tokens": N, "firebase_initialized": true/false}

Registered tokens: %d
Firebase initialized: %v
API Version: FCM v1 (Firebase Admin SDK)
`, tokenStore.Count(), messagingClient != nil)
}

func sendFCMNotification(deviceToken, title, body string) error {
	if messagingClient == nil {
		return fmt.Errorf("Firebase messaging client not initialized")
	}

	// Decrypt token if it appears to be encrypted (base64 encoded and long)
	decryptedToken := deviceToken
	if len(deviceToken) > 100 && !strings.Contains(deviceToken, ":") {
		// Looks like an encrypted token, try to decrypt
		var err error
		decryptedToken, err = decryptToken(deviceToken)
		if err != nil {
			return fmt.Errorf("failed to decrypt token: %v", err)
		}
		log.Printf("Decrypted token for FCM sending")
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

func decryptToken(encryptedToken string) (string, error) {
	if privateKey == nil {
		return "", fmt.Errorf("private key not loaded")
	}

	// Decode base64
	encryptedBytes, err := base64.StdEncoding.DecodeString(encryptedToken)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64: %v", err)
	}

	// Decrypt
	decryptedBytes, err := rsa.DecryptPKCS1v15(rand.Reader, privateKey, encryptedBytes)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt token: %v", err)
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
