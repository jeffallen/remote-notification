package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
)

const (
	notificationBackendURL = "http://192.168.1.141:8080"
)

type TokenRegistration struct {
	Token    string `json:"token"`
	Platform string `json:"platform"`
}

type NotificationRequest struct {
	Token string `json:"token"`
	Title string `json:"title"`
	Body  string `json:"body"`
}

// TokenStore holds device tokens in memory only
// Deliberately separate from any user data for privacy
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
	log.Printf("Token stored: %s (total: %d)", tokenPreview, len(ts.tokens))
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
	http.HandleFunc("/send-all", handleSendAll)
	http.HandleFunc("/", handleHome)

	port := "8081"
	log.Printf("App Backend Server starting on port %s", port)
	log.Printf("Forwarding to notification backend: %s", notificationBackendURL)
	log.Printf("Web interface available at: http://192.168.1.141:%s", port)

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

	// Store token in memory (privacy: no user data association)
	tokenStore.AddToken(reg.Token)

	// Forward to notification backend
	if err := forwardTokenToBackend(reg); err != nil {
		log.Printf("Failed to forward token to backend: %v", err)
		// Still return success to app since we stored it
	}

	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"success":      true,
		"message":      "Token registered successfully",
		"platform":     reg.Platform,
		"total_tokens": tokenStore.Count(),
	}
	json.NewEncoder(w).Encode(response)
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

	tokens := tokenStore.GetTokens()
	if len(tokens) == 0 {
		http.Error(w, "No tokens registered", http.StatusBadRequest)
		return
	}

	successCount := 0
	errorCount := 0

	// Send individual notification for each token
	for _, token := range tokens {
		notifReq := NotificationRequest{
			Token: token,
			Title: "App Notification",
			Body:  message,
		}

		if err := sendNotificationToBackend(notifReq); err != nil {
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
	t.Execute(w, data)
}

func forwardTokenToBackend(reg TokenRegistration) error {
	data, err := json.Marshal(reg)
	if err != nil {
		return fmt.Errorf("failed to marshal token: %v", err)
	}

	resp, err := http.Post(notificationBackendURL+"/register", "application/json", bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("failed to post to backend: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("backend returned %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func sendNotificationToBackend(notifReq NotificationRequest) error {
	// Create the payload that notification-backend expects on /notify endpoint
	// Note: We need to create this endpoint on notification-backend
	payload := map[string]string{
		"token": notifReq.Token,
		"title": notifReq.Title,
		"body":  notifReq.Body,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %v", err)
	}

	resp, err := http.Post(notificationBackendURL+"/notify", "application/json", bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("failed to post to backend: %v", err)
	}
	defer resp.Body.Close()

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
        <p><small>Tokens stored in memory only, no user data association</small></p>
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
            <li>Device tokens stored in RAM only (lost on restart)</li>
            <li>No association with user accounts or personal data</li>
            <li>Forwards tokens to notification backend without storing user context</li>
            <li>Individual notification requests preserve token separation</li>
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
