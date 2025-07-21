package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

var (
	// Command-line configuration
	port                   = flag.String("port", "8443", "Port to listen on")
	certFile               = flag.String("cert", "cert.pem", "Path to TLS certificate file")
	keyFile                = flag.String("key", "key.pem", "Path to TLS private key file")
	publicKeyPath          = flag.String("public-key", "public_key.pem", "Path to RSA public key file")
	notificationBackendURL = flag.String("backend-url", "http://localhost:8080", "URL of the notification backend service")
	version                = "dev" // Set by build flags
)

type TokenRegistration struct {
	EncryptedData string `json:"encrypted_data"`
	Platform      string `json:"platform"`
}

type NotificationRequest struct {
	TokenID       string `json:"token_id"`
	PublicKeyHash string `json:"public_key_hash,omitempty"`
	Title         string `json:"title"`
	Body          string `json:"body"`
}

// TokenStore holds opaque token identifiers in memory only
// Deliberately separate from any user data for privacy
type TokenStore struct {
	mu       sync.RWMutex
	tokenIDs map[string]time.Time // opaque_token_id -> registration time
}

func NewTokenStore() *TokenStore {
	return &TokenStore{
		tokenIDs: make(map[string]time.Time),
	}
}

func (ts *TokenStore) AddTokenID(tokenID string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.tokenIDs[tokenID] = time.Now()

	// Safe to log opaque IDs (they reveal nothing about actual tokens)
	log.Printf("Opaque token ID stored: %s...%s (total: %d)",
		tokenID[:8], tokenID[len(tokenID)-8:], len(ts.tokenIDs))
}

func (ts *TokenStore) GetTokenIDs() []string {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	tokenIDs := make([]string, 0, len(ts.tokenIDs))
	for tokenID := range ts.tokenIDs {
		tokenIDs = append(tokenIDs, tokenID)
	}
	return tokenIDs
}

func (ts *TokenStore) Count() int {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	return len(ts.tokenIDs)
}

var (
	tokenStore    = NewTokenStore()
	publicKeyHash string
)

func main() {
	flag.Parse()

	log.Printf("App Backend Server v%s", version)
	log.Printf("Configuration:")
	log.Printf("  Port: %s", *port)
	log.Printf("  TLS Cert: %s", *certFile)
	log.Printf("  TLS Key: %s", *keyFile)
	log.Printf("  Public Key: %s", *publicKeyPath)
	log.Printf("  Backend URL: %s", *notificationBackendURL)
	
	// Load public key and compute hash
	publicKeyPEM, err := readPublicKeyPEM(*publicKeyPath)
	if err != nil {
		log.Fatalf("Error loading public key: %v", err)
	}
	publicKeyHash = computePublicKeyHash(publicKeyPEM)
	log.Printf("Public key hash computed: %s", publicKeyHash[:16]+"...")

	http.HandleFunc("/register", handleRegister)
	http.HandleFunc("/send-all", handleSendAll)
	http.HandleFunc("/", handleHome)

	log.Printf("App Backend Server starting on HTTPS port %s", *port)
	log.Printf("Web interface available at: https://localhost:%s", *port)
	log.Printf("Android emulator can access at: https://10.0.2.2:%s/", *port)

	if err := http.ListenAndServeTLS(":"+*port, *certFile, *keyFile, nil); err != nil {
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

	// Forward to notification backend first to get opaque ID
	opaqueID, err := forwardTokenToBackend(reg)
	if err != nil {
		log.Printf("Failed to forward encrypted data to backend: %v", err)
		http.Error(w, "Failed to register token with backend", http.StatusInternalServerError)
		return
	}

	// Store opaque ID in memory (privacy: no user data association, opaque identifier)
	tokenStore.AddTokenID(opaqueID)

	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"success":      true,
		"message":      "Encrypted token registered successfully",
		"platform":     reg.Platform,
		"total_tokens": tokenStore.Count(),
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding response: %v", err)
	}
}

func handleSendAll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	message := r.FormValue("message")
	if message == "" {
		http.Error(w, "Message is required", http.StatusBadRequest)
		return
	}

	tokenIDs := tokenStore.GetTokenIDs()
	if len(tokenIDs) == 0 {
		http.Error(w, "No tokens registered", http.StatusBadRequest)
		return
	}

	successCount := 0
	errorCount := 0

	// Send individual notification for each token ID
	for _, tokenID := range tokenIDs {
		notifReq := NotificationRequest{
			TokenID:       tokenID,
			PublicKeyHash: publicKeyHash,
			Title:         "App Notification",
			Body:          message,
		}

		if err := sendNotificationToBackend(notifReq); err != nil {
			log.Printf("Failed to send to token ID %s...%s: %v",
				tokenID[:8], tokenID[len(tokenID)-8:], err)
			errorCount++
		} else {
			successCount++
		}
	}

	// Redirect back to home with results
	http.Redirect(w, r, fmt.Sprintf("/?sent=%d&errors=%d", successCount, errorCount), http.StatusSeeOther)
}

func handleHome(w http.ResponseWriter, r *http.Request) {
	data := struct {
		TokenCount  int
		SentCount   string
		ErrorCount  string
		ShowResults bool
	}{
		TokenCount:  tokenStore.Count(),
		SentCount:   r.URL.Query().Get("sent"),
		ErrorCount:  r.URL.Query().Get("errors"),
		ShowResults: r.URL.Query().Get("sent") != "",
	}

	t := template.Must(template.New("home").Parse(homeTemplate))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.Execute(w, data); err != nil {
		log.Printf("Error executing template: %v", err)
	}
}

func forwardTokenToBackend(reg TokenRegistration) (string, error) {
	data, err := json.Marshal(reg)
	if err != nil {
		return "", fmt.Errorf("failed to marshal token: %v", err)
	}

	resp, err := http.Post(*notificationBackendURL+"/register", "application/json", bytes.NewBuffer(data))
	if err != nil {
		return "", fmt.Errorf("failed to post to backend: %v", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			log.Printf("Error closing response body: %v", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("backend returned %d: %s", resp.StatusCode, string(body))
	}

	// Parse response to get opaque token ID
	var response struct {
		Success bool   `json:"success"`
		TokenID string `json:"token_id"`
		Message string `json:"message"`
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %v", err)
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("failed to parse response: %v", err)
	}

	if !response.Success || response.TokenID == "" {
		return "", fmt.Errorf("backend registration failed: %s", response.Message)
	}

	return response.TokenID, nil
}

func sendNotificationToBackend(notifReq NotificationRequest) error {
	// Create the payload that notification-backend expects on /notify endpoint
	payload := map[string]string{
		"token_id": notifReq.TokenID,
		"title":    notifReq.Title,
		"body":     notifReq.Body,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %v", err)
	}

	resp, err := http.Post(*notificationBackendURL+"/notify", "application/json", bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("failed to post to backend: %v", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			log.Printf("Error closing response body: %v", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("backend returned %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

const homeTemplate = `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>App Backend - Notification Service</title>
    <style>
        body { font-family: Arial, sans-serif; max-width: 800px; margin: 0 auto; padding: 20px; }
        .header { background: #f5f5f5; padding: 20px; border-radius: 8px; margin-bottom: 20px; }
        .stats { background: #e8f4fd; padding: 15px; border-radius: 8px; margin-bottom: 20px; }
        .send-form { background: #f8f9fa; padding: 20px; border-radius: 8px; }
        .results { background: #d4edda; padding: 15px; border-radius: 8px; margin-bottom: 20px; border: 1px solid #c3e6cb; }
        .error-results { background: #f8d7da; border: 1px solid #f5c6cb; }
        textarea { width: 100%; height: 100px; margin: 10px 0; padding: 10px; border: 1px solid #ddd; border-radius: 4px; }
        button { background: #007bff; color: white; padding: 10px 20px; border: none; border-radius: 4px; cursor: pointer; font-size: 16px; }
        button:hover { background: #0056b3; }
        button:disabled { background: #6c757d; cursor: not-allowed; }
        .privacy-note { background: #fff3cd; padding: 15px; border-radius: 8px; margin-top: 20px; border: 1px solid #ffeaa7; }
    </style>
</head>
<body>
    <div class="header">
        <h1>App Backend - Notification Service</h1>
        <p>Intermediate server for privacy-separated device token management</p>
    </div>

    <div class="stats">
        <h2>📱 Device Tokens</h2>
        <p><strong>{{.TokenCount}}</strong> device tokens currently registered</p>
        <p><small>Opaque token IDs stored in memory only, no user data association</small></p>
    </div>

    {{if .ShowResults}}
    <div class="results {{if ne .ErrorCount "0"}}error-results{{end}}">
        <h3>📤 Notification Results</h3>
        <p>✅ Successfully sent to <strong>{{.SentCount}}</strong> devices</p>
        {{if ne .ErrorCount "0"}}
        <p>❌ Failed to send to <strong>{{.ErrorCount}}</strong> devices</p>
        {{end}}
    </div>
    {{end}}

    <div class="send-form">
        <h2>📢 Send Notification to All Devices</h2>
        {{if gt .TokenCount 0}}
        <form method="post" action="/send-all">
            <label for="message">Message:</label>
            <textarea name="message" id="message" placeholder="Enter your notification message here..." required></textarea>
            <button type="submit">Send to All {{.TokenCount}} Devices</button>
        </form>
        {{else}}
        <p>No devices registered yet. Register some tokens first.</p>
        <button disabled>Send to All (No Devices)</button>
        {{end}}
    </div>

    <div class="privacy-note">
        <h3>🔒 Privacy Design</h3>
        <ul>
            <li>Only opaque token IDs stored in RAM (lost on restart)</li>
            <li>No association with user accounts or personal data</li>
            <li>Actual encrypted tokens stored only in notification backend</li>
            <li>App backend cannot decrypt or access actual device tokens</li>
            <li>Individual notification requests use opaque identifiers</li>
        </ul>
    </div>

    <script>
        // Auto-refresh token count every 30 seconds
        setTimeout(function() {
            if (!window.location.search.includes('sent=')) {
                window.location.reload();
            }
        }, 30000);
    </script>
</body>
</html>
`

// readPublicKeyPEM reads a public key PEM file and returns its content
func readPublicKeyPEM(keyPath string) (string, error) {
	data, err := os.ReadFile(keyPath)
	if err != nil {
		return "", fmt.Errorf("failed to read public key file: %v", err)
	}
	return string(data), nil
}

// computePublicKeyHash computes a SHA256 hash of the public key for use in storage keys
func computePublicKeyHash(publicKeyPEM string) string {
	hash := sha256.Sum256([]byte(publicKeyPEM))
	return hex.EncodeToString(hash[:])
}
