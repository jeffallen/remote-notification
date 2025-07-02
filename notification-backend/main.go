package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
)

const (
	fcmURL    = "https://fcm.googleapis.com/fcm/send"
	serverKey = "YOUR_FCM_SERVER_KEY_HERE" // Replace with your actual FCM server key
)

type TokenRegistration struct {
	Token    string `json:"token"`
	Platform string `json:"platform"`
}

type FCMMessage struct {
	To           string            `json:"to"`
	Notification map[string]string `json:"notification"`
}

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

var tokenStore = NewTokenStore()

func main() {
	http.HandleFunc("/register", handleRegister)
	http.HandleFunc("/send", handleSend)
	http.HandleFunc("/notify", handleNotify)
	http.HandleFunc("/status", handleStatus)
	http.HandleFunc("/", handleRoot)

	port := ":8080"
	log.Printf("FCM Notification Server starting on port %s", port)
	log.Printf("Endpoints:")
	log.Printf("  POST /register - Register FCM token")
	log.Printf("  POST /send     - Send notification to all registered tokens")
	log.Printf("  POST /notify   - Send notification to specific token")
	log.Printf("  GET  /status   - Show registered token count")
	log.Printf("  GET  /         - Show this help")

	if err := http.ListenAndServe(port, nil); err != nil {
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
		"registered_tokens":     tokenStore.Count(),
		"server_key_configured": serverKey != "YOUR_FCM_SERVER_KEY_HERE",
	}
	json.NewEncoder(w).Encode(response)
}

func handleRoot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintf(w, `FCM Notification Server

Endpoints:
  POST /register - Register FCM token
    Body: {"token": "fcm-token", "platform": "android"}

  POST /send - Send notification to all registered tokens
    Body: {"title": "Hello", "body": "Test message"}

  POST /notify - Send notification to specific token
    Body: {"token": "fcm-token", "title": "Hello", "body": "Test message"}

  GET /status - Show server status
    Returns: {"registered_tokens": N, "server_key_configured": true/false}

Registered tokens: %d
Server key configured: %v
`, tokenStore.Count(), serverKey != "YOUR_FCM_SERVER_KEY_HERE")
}

func sendFCMNotification(deviceToken, title, body string) error {
	if serverKey == "YOUR_FCM_SERVER_KEY_HERE" {
		return fmt.Errorf("FCM server key not configured")
	}

	message := FCMMessage{
		To: deviceToken,
		Notification: map[string]string{
			"title": title,
			"body":  body,
		},
	}

	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %v", err)
	}

	req, err := http.NewRequest("POST", fcmURL, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Authorization", "key="+serverKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("FCM request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}
