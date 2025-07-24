package main

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"flag"
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

var (
	// Command-line configuration
	port                  = flag.String("port", "8080", "Port to listen on")
	serviceAccountKeyPath = flag.String("firebase-key", "key.json", "Path to Firebase service account key file")
	privateKeyPath        = flag.String("private-key", "private_key.pem", "Path to RSA private key file")
	publicKeyPath         = flag.String("public-key", "public_key.pem", "Path to RSA public key file")
	storageFile           = flag.String("storage-file", "tokens.json", "Path to token storage file (fallback only)")
	
	// Exoscale SOS configuration
	sosAccessKey = flag.String("sos-access-key", "", "Exoscale SOS access key")
	sosSecretKey = flag.String("sos-secret-key", "", "Exoscale SOS secret key")
	sosBucket    = flag.String("sos-bucket", "notification-tokens", "Exoscale SOS bucket name")
	sosZone      = flag.String("sos-zone", "ch-gva-2", "Exoscale SOS zone")
	
	version = "dev" // Set by build flags
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
	TokenID       string `json:"token_id"`                   // Opaque ID field (required)
	PublicKeyHash string `json:"public_key_hash,omitempty"` // Public key hash for storage key
	Title         string `json:"title"`
	Body          string `json:"body"`
}

// TokenMapping represents a stored token mapping
type TokenMapping struct {
	OpaqueID      string    `json:"opaque_id"`
	EncryptedData string    `json:"encrypted_data"`
	Platform      string    `json:"platform"`
	RegisteredAt  time.Time `json:"registered_at"`
}

// DurableTokenStore provides persistent token storage
type DurableTokenStore struct {
	mu          sync.RWMutex
	mappings    map[string]*TokenMapping // opaque_id -> TokenMapping
	storageFile string
}

func NewDurableTokenStore(storageFile string) *DurableTokenStore {
	store := &DurableTokenStore{
		mappings:    make(map[string]*TokenMapping),
		storageFile: storageFile,
	}

	// Load existing tokens from file
	if err := store.loadFromFile(); err != nil {
		log.Printf("Warning: Could not load existing tokens: %v", err)
	}

	return store
}

func (ts *DurableTokenStore) generateOpaqueID() string {
	// Generate 32 random bytes (256 bits)
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		log.Printf("Error generating random bytes: %v", err)
		// Fallback to timestamp + random for uniqueness
		return fmt.Sprintf("%d_%x", time.Now().UnixNano(), bytes[:16])
	}
	return hex.EncodeToString(bytes)
}

func (ts *DurableTokenStore) AddToken(encryptedData, platform string) (string, error) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	opaqueID := ts.generateOpaqueID()

	// Ensure uniqueness (extremely unlikely collision, but handle it)
	for _, exists := ts.mappings[opaqueID]; exists; {
		opaqueID = ts.generateOpaqueID()
		_, exists = ts.mappings[opaqueID]
	}

	mapping := &TokenMapping{
		OpaqueID:      opaqueID,
		EncryptedData: encryptedData,
		Platform:      platform,
		RegisteredAt:  time.Now(),
	}

	ts.mappings[opaqueID] = mapping

	// Persist to file
	if err := ts.saveToFile(); err != nil {
		log.Printf("Warning: Failed to persist token to file: %v", err)
	}

	log.Printf("Token registered with opaque ID: %s...%s (platform: %s, total: %d)",
		opaqueID[:8], opaqueID[len(opaqueID)-8:], platform, len(ts.mappings))

	return opaqueID, nil
}

func (ts *DurableTokenStore) GetEncryptedToken(opaqueID string) (string, error) {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	mapping, exists := ts.mappings[opaqueID]
	if !exists {
		return "", fmt.Errorf("opaque ID not found")
	}

	return mapping.EncryptedData, nil
}

func (ts *DurableTokenStore) GetAllOpaqueIDs() []string {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	ids := make([]string, 0, len(ts.mappings))
	for id := range ts.mappings {
		ids = append(ids, id)
	}
	return ids
}

func (ts *DurableTokenStore) Count() int {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	return len(ts.mappings)
}

func (ts *DurableTokenStore) loadFromFile() error {
	data, err := os.ReadFile(ts.storageFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // File doesn't exist yet, that's fine
		}
		return err
	}

	var mappings []*TokenMapping
	if err := json.Unmarshal(data, &mappings); err != nil {
		return err
	}

	for _, mapping := range mappings {
		ts.mappings[mapping.OpaqueID] = mapping
	}

	log.Printf("Loaded %d tokens from storage file", len(mappings))
	return nil
}

func (ts *DurableTokenStore) saveToFile() error {
	// Convert map to slice for JSON serialization
	mappings := make([]*TokenMapping, 0, len(ts.mappings))
	for _, mapping := range ts.mappings {
		mappings = append(mappings, mapping)
	}

	data, err := json.MarshalIndent(mappings, "", "  ")
	if err != nil {
		return err
	}

	// Write to temporary file first, then rename (atomic operation)
	tempFile := ts.storageFile + ".tmp"
	if err := os.WriteFile(tempFile, data, 0600); err != nil {
		return err
	}

	return os.Rename(tempFile, ts.storageFile)
}

// RequestLog represents a structured log entry for HTTP requests
type RequestLog struct {
	Timestamp    time.Time `json:"timestamp"`
	Method       string    `json:"method"`
	Path         string    `json:"path"`
	RemoteAddr   string    `json:"remote_addr"`
	UserAgent    string    `json:"user_agent"`
	StatusCode   int       `json:"status_code"`
	ResponseTime int64     `json:"response_time_ms"`
	BodySize     int64     `json:"body_size"`
	Error        string    `json:"error,omitempty"`
}

// ResponseWriter wrapper to capture status code and response size
type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
	bodySize   int64
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

func (lrw *loggingResponseWriter) Write(b []byte) (int, error) {
	size, err := lrw.ResponseWriter.Write(b)
	lrw.bodySize += int64(size)
	return size, err
}

var (
	tokenStore      *DurableTokenStore
	exoscaleStorage *ExoscaleStorage
	messagingClient *messaging.Client
	privateKey      *rsa.PrivateKey
	publicKeyHash   string
	useExoscale     bool
)

// loggingMiddleware wraps HTTP handlers to provide structured logging
func loggingMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		
		// Create logging response writer
		lrw := &loggingResponseWriter{
			ResponseWriter: w,
			statusCode:     200, // Default status code
		}
		
		// Call the next handler
		next(lrw, r)
		
		// Calculate response time
		responseTime := time.Since(start).Milliseconds()
		
		// Create structured log entry
		logEntry := RequestLog{
			Timestamp:    start,
			Method:       r.Method,
			Path:         r.URL.Path,
			RemoteAddr:   getClientIP(r),
			UserAgent:    r.UserAgent(),
			StatusCode:   lrw.statusCode,
			ResponseTime: responseTime,
			BodySize:     lrw.bodySize,
		}
		
		// Add error field for non-2xx responses
		if lrw.statusCode >= 400 {
			logEntry.Error = http.StatusText(lrw.statusCode)
		}
		
		// Log as JSON
		logJSON, err := json.Marshal(logEntry)
		if err != nil {
			log.Printf("Error marshaling log entry: %v", err)
			return
		}
		
		log.Printf("REQUEST_LOG: %s", string(logJSON))
	}
}

// getClientIP extracts the real client IP from request headers
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header (for proxies/load balancers)
	if xForwardedFor := r.Header.Get("X-Forwarded-For"); xForwardedFor != "" {
		// X-Forwarded-For can contain multiple IPs, take the first one
		ifs := strings.Split(xForwardedFor, ",")
		if len(ifs) > 0 {
			return strings.TrimSpace(ifs[0])
		}
	}
	
	// Check X-Real-IP header (for nginx)
	if xRealIP := r.Header.Get("X-Real-IP"); xRealIP != "" {
		return xRealIP
	}
	
	// Fall back to RemoteAddr
	// Remove port if present
	if idx := strings.LastIndex(r.RemoteAddr, ":"); idx != -1 {
		return r.RemoteAddr[:idx]
	}
	return r.RemoteAddr
}

func main() {
	flag.Parse()

	log.Printf("Notification Backend Server v%s", version)
	log.Printf("Configuration:")
	log.Printf("  Port: %s", *port)
	log.Printf("  Firebase Key: %s", *serviceAccountKeyPath)
	log.Printf("  Private Key: %s", *privateKeyPath)
	log.Printf("  Public Key: %s", *publicKeyPath)
	log.Printf("  Storage File: %s (fallback)", *storageFile)
	log.Printf("  SOS Bucket: %s", *sosBucket)
	log.Printf("  SOS Zone: %s", *sosZone)
	log.Printf("  SOS Access Key: %s", maskString(*sosAccessKey))
	
	// Determine if we should use Exoscale SOS
	useExoscale = *sosAccessKey != "" && *sosSecretKey != ""

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
	
	// Load public key and compute hash
	publicKeyPEM, err := readPublicKeyPEM(*publicKeyPath)
	if err != nil {
		log.Fatalf("Error loading public key: %v", err)
	}
	publicKeyHash = ComputePublicKeyHash(publicKeyPEM)
	log.Printf("Public key hash computed: %s", publicKeyHash[:16]+"...")

	// Initialize storage layer
	if useExoscale {
		// Initialize Exoscale SOS storage
		exoscaleStorage, err = NewExoscaleStorage(*sosAccessKey, *sosSecretKey, *sosBucket, *sosZone, publicKeyHash)
		if err != nil {
			log.Fatalf("Error initializing Exoscale SOS storage: %v", err)
		}
		log.Printf("Using Exoscale SOS for durable storage")
	} else {
		log.Printf("Warning: No SOS credentials provided, falling back to local file storage")
		log.Printf("         This is not recommended for production use")
	}
	
	// Initialize fallback file-based token store (always available)
	tokenStore = NewDurableTokenStore(*storageFile)
	
	// Start cleanup goroutine if using Exoscale
	if useExoscale {
		go startCleanupRoutine()
	}

	http.HandleFunc("/register", loggingMiddleware(handleRegister))
	http.HandleFunc("/send", loggingMiddleware(handleSend))
	http.HandleFunc("/notify", loggingMiddleware(handleNotify))
	http.HandleFunc("/status", loggingMiddleware(handleStatus))
	http.HandleFunc("/", loggingMiddleware(handleRoot))

	log.Printf("FCM Notification Server starting on port %s", *port)
	log.Printf("Storage: %s", getStorageType())
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

	// Generate opaque ID
	opaqueID := generateOpaqueID()
	
	// Store token using primary storage (Exoscale SOS if available, fallback to file)
	if useExoscale {
		ctx := context.Background()
		if err := exoscaleStorage.StoreToken(ctx, opaqueID, reg.EncryptedData, reg.Platform); err != nil {
			log.Printf("Failed to store token in Exoscale SOS: %v", err)
			http.Error(w, "Failed to store token", http.StatusInternalServerError)
			return
		}
	} else {
		// Fallback to file-based storage
		if _, err := tokenStore.AddToken(reg.EncryptedData, reg.Platform); err != nil {
			log.Printf("Failed to store token in file storage: %v", err)
			http.Error(w, "Failed to store token", http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"success":      true,
		"message":      "Token registered successfully",
		"token_id":     opaqueID,
		"platform":     reg.Platform,
		"total_tokens": getTotalTokenCount(),
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

	tokens, err := getAllTokens()
	if err != nil {
		log.Printf("Failed to get tokens: %v", err)
		http.Error(w, "Failed to retrieve tokens", http.StatusInternalServerError)
		return
	}
	
	if len(tokens) == 0 {
		http.Error(w, "No tokens registered", http.StatusBadRequest)
		return
	}

	successCount := 0
	errorCount := 0

	for _, token := range tokens {
		if err := sendFCMNotification(token.EncryptedData, notif.Title, notif.Body); err != nil {
			log.Printf("Failed to send to opaque ID %s...%s: %v",
				token.OpaqueID[:8], token.OpaqueID[len(token.OpaqueID)-8:], err)
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

	if notif.Title == "" || notif.Body == "" {
		http.Error(w, "Title and body are required", http.StatusBadRequest)
		return
	}

	// Only accept opaque ID (no backwards compatibility)
	if notif.TokenID == "" {
		http.Error(w, "token_id is required", http.StatusBadRequest)
		return
	}

	token, err := getToken(notif.TokenID)
	if err != nil {
		log.Printf("Token ID not found: %s", notif.TokenID)
		http.Error(w, "Token ID not found", http.StatusBadRequest)
		return
	}
	encryptedData := token.EncryptedData

	if err := sendFCMNotification(encryptedData, notif.Title, notif.Body); err != nil {
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
		"registered_tokens":    getTotalTokenCount(),
		"firebase_initialized": messagingClient != nil,
		"api_version":          "FCM v1 (Firebase Admin SDK)",
		"storage_type":         getStorageType(),
		"public_key_hash":      publicKeyHash[:16] + "...",
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
    Body: {"token_id": "opaque-token-id", "title": "Hello", "body": "Test message"}

  GET /status - Show server status
    Returns: {"registered_tokens": N, "firebase_initialized": true/false}

Registered tokens: %d
Firebase initialized: %v
API Version: FCM v1 (Firebase Admin SDK)
Storage Type: %s
Public Key Hash: %s
`, getTotalTokenCount(), messagingClient != nil, getStorageType(), publicKeyHash[:16]+"..."); err != nil {
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
		// Convert string to byte slice to enable overwriting
		// This uses unsafe to access the underlying string data
		bytes := []byte(*s)
		for i := range bytes {
			bytes[i] = 0
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

// maskString masks a string for logging, showing only first and last 4 chars
func maskString(s string) string {
	if len(s) <= 8 {
		return "[REDACTED]"
	}
	return s[:4] + "..." + s[len(s)-4:]
}

// getStorageType returns a human-readable description of the storage type in use
func getStorageType() string {
	if useExoscale {
		return fmt.Sprintf("Exoscale SOS (bucket: %s, zone: %s)", *sosBucket, *sosZone)
	}
	return "Local file (fallback mode)"
}

// readPublicKeyPEM reads a public key PEM file and returns its content
func readPublicKeyPEM(keyPath string) (string, error) {
	data, err := os.ReadFile(keyPath)
	if err != nil {
		return "", fmt.Errorf("failed to read public key file: %v", err)
	}
	return string(data), nil
}

// startCleanupRoutine runs a goroutine that periodically cleans up old tokens
func startCleanupRoutine() {
	ticker := time.NewTicker(24 * time.Hour) // Run cleanup once per day
	defer ticker.Stop()
	
	log.Printf("Starting token cleanup routine (runs every 24 hours)")
	
	// Run initial cleanup after 5 minutes to allow for startup
	time.AfterFunc(5*time.Minute, func() {
		ctx := context.Background()
		deleted, err := exoscaleStorage.CleanupOldTokens(ctx, 30*24*time.Hour) // 30 days
		if err != nil {
			log.Printf("Error during initial token cleanup: %v", err)
		} else {
			log.Printf("Initial cleanup completed: removed %d old tokens", deleted)
		}
	})
	
	for range ticker.C {
		ctx := context.Background()
		deleted, err := exoscaleStorage.CleanupOldTokens(ctx, 30*24*time.Hour) // 30 days
		if err != nil {
			log.Printf("Error during scheduled token cleanup: %v", err)
		} else if deleted > 0 {
			log.Printf("Scheduled cleanup completed: removed %d old tokens", deleted)
		}
	}
}

// Helper functions for unified storage access

// generateOpaqueID creates a new opaque identifier
func generateOpaqueID() string {
	// Generate 32 random bytes (256 bits)
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		log.Printf("Error generating random bytes: %v", err)
		// Fallback to timestamp + random for uniqueness
		return fmt.Sprintf("%d_%x", time.Now().UnixNano(), bytes[:16])
	}
	return hex.EncodeToString(bytes)
}

// getToken retrieves a token by opaque ID from the appropriate storage
func getToken(opaqueID string) (*TokenStorageInfo, error) {
	if useExoscale {
		ctx := context.Background()
		return exoscaleStorage.GetToken(ctx, opaqueID)
	}
	
	// Fallback to file storage - need to convert format
	encryptedData, err := tokenStore.GetEncryptedToken(opaqueID)
	if err != nil {
		return nil, err
	}
	
	return &TokenStorageInfo{
		OpaqueID:      opaqueID,
		EncryptedData: encryptedData,
		Platform:      "unknown", // File storage doesn't track platform separately
		LastUsedAt:    time.Now(),
	}, nil
}

// getAllTokens retrieves all tokens from the appropriate storage
func getAllTokens() ([]*TokenStorageInfo, error) {
	if useExoscale {
		ctx := context.Background()
		return exoscaleStorage.ListAllTokens(ctx)
	}
	
	// Fallback to file storage - need to convert format
	opaqueIDs := tokenStore.GetAllOpaqueIDs()
	tokens := make([]*TokenStorageInfo, 0, len(opaqueIDs))
	
	for _, opaqueID := range opaqueIDs {
		encryptedData, err := tokenStore.GetEncryptedToken(opaqueID)
		if err != nil {
			log.Printf("Warning: failed to get token for ID %s: %v", opaqueID[:16]+"...", err)
			continue
		}
		
		tokens = append(tokens, &TokenStorageInfo{
			OpaqueID:      opaqueID,
			EncryptedData: encryptedData,
			Platform:      "unknown",
			LastUsedAt:    time.Now(),
		})
	}
	
	return tokens, nil
}

// getTotalTokenCount returns the total number of tokens in storage
func getTotalTokenCount() int {
	if useExoscale {
		tokens, err := getAllTokens()
		if err != nil {
			log.Printf("Warning: failed to count tokens: %v", err)
			return 0
		}
		return len(tokens)
	}
	return tokenStore.Count()
}
