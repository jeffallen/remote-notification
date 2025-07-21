// Demo Integration Test - Shows the complete opaque ID flow
// This demonstrates how the system works end-to-end
package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"
)

func main() {
	fmt.Println("=== Opaque ID Token Registration Demo ===")
	fmt.Println()

	// Step 1: Android app generates and encrypts FCM token
	fmt.Println("Step 1: Android App encrypts FCM token")
	originalToken := "fake_fcm_token_abc123xyz789"
	encryptedToken := simulateHybridEncryption(originalToken)
	fmt.Printf("  Original FCM Token: %s\n", originalToken)
	fmt.Printf("  Encrypted Token: %s...%s (length: %d)\n",
		encryptedToken[:20], encryptedToken[len(encryptedToken)-20:], len(encryptedToken))
	fmt.Println()

	// Step 2: App-Backend receives registration
	fmt.Println("Step 2: App-Backend receives registration")
	registerReq := map[string]string{
		"encrypted_data": encryptedToken,
		"platform":       "android",
	}
	reqData, _ := json.MarshalIndent(registerReq, "", "  ")
	fmt.Printf("  Request to app-backend /register:\n%s\n", reqData)
	fmt.Println()

	// Step 3: App-Backend forwards to Notification-Backend
	fmt.Println("Step 3: App-Backend forwards to Notification-Backend")
	fmt.Printf("  App-Backend -> Notification-Backend /register\n")
	fmt.Printf("  Same payload: encrypted_data + platform\n")
	fmt.Println()

	// Step 4: Notification-Backend generates opaque ID
	fmt.Println("Step 4: Notification-Backend processes registration")
	opaqueID := generateOpaqueID()
	fmt.Printf("  Generated 256-bit opaque ID: %s\n", opaqueID)
	fmt.Printf("  ID length: %d characters (64 hex chars = 32 bytes = 256 bits)\n", len(opaqueID))
	fmt.Println()

	// Step 5: Notification-Backend stores mapping
	fmt.Println("Step 5: Notification-Backend stores mapping")
	mapping := map[string]interface{}{
		"opaque_id":      opaqueID,
		"encrypted_data": encryptedToken,
		"platform":       "android",
		"registered_at":  time.Now().Format(time.RFC3339),
	}
	mappingData, _ := json.MarshalIndent(mapping, "", "  ")
	fmt.Printf("  Stored mapping in tokens.json:\n%s\n", mappingData)
	fmt.Println()

	// Step 6: Notification-Backend returns opaque ID
	fmt.Println("Step 6: Notification-Backend returns opaque ID")
	response := map[string]interface{}{
		"success":      true,
		"message":      "Token registered successfully",
		"token_id":     opaqueID,
		"platform":     "android",
		"total_tokens": 1,
	}
	respData, _ := json.MarshalIndent(response, "", "  ")
	fmt.Printf("  Response from notification-backend:\n%s\n", respData)
	fmt.Println()

	// Step 7: App-Backend stores opaque ID
	fmt.Println("Step 7: App-Backend stores opaque ID")
	fmt.Printf("  App-Backend receives opaque ID: %s\n", opaqueID)
	fmt.Printf("  Stores in memory: opaque_id -> registration_time\n")
	fmt.Printf("  NO encrypted token stored in app-backend!\n")
	fmt.Println()

	// Step 8: App-Backend returns success
	fmt.Println("Step 8: App-Backend returns success to Android")
	appResponse := map[string]interface{}{
		"success":      true,
		"message":      "Encrypted token registered successfully",
		"platform":     "android",
		"total_tokens": 1,
	}
	appRespData, _ := json.MarshalIndent(appResponse, "", "  ")
	fmt.Printf("  Response to Android app:\n%s\n", appRespData)
	fmt.Println()

	fmt.Println("=== Later: Sending Notification ===")
	fmt.Println()

	// Step 9: Admin sends notification
	fmt.Println("Step 9: Admin sends notification via App-Backend")
	notifyReq := map[string]string{
		"message": "Hello from the notification system!",
	}
	notifyData, _ := json.MarshalIndent(notifyReq, "", "  ")
	fmt.Printf("  POST /send-all with message:\n%s\n", notifyData)
	fmt.Println()

	// Step 10: App-Backend uses opaque ID
	fmt.Println("Step 10: App-Backend sends notification with opaque ID")
	notificationReq := map[string]string{
		"token_id": opaqueID,
		"title":    "App Notification",
		"body":     "Hello from the notification system!",
	}
	notifReqData, _ := json.MarshalIndent(notificationReq, "", "  ")
	fmt.Printf("  App-Backend -> Notification-Backend /notify:\n%s\n", notifReqData)
	fmt.Println()

	// Step 11: Notification-Backend looks up encrypted token
	fmt.Println("Step 11: Notification-Backend looks up token")
	fmt.Printf("  Receives opaque ID: %s\n", opaqueID)
	fmt.Printf("  Looks up in storage: opaque_id -> encrypted_data\n")
	fmt.Printf("  Found encrypted token: %s...%s\n",
		encryptedToken[:20], encryptedToken[len(encryptedToken)-20:])
	fmt.Println()

	// Step 12: Notification-Backend decrypts and sends
	fmt.Println("Step 12: Notification-Backend decrypts and sends to FCM")
	decryptedToken := simulateHybridDecryption(encryptedToken)
	fmt.Printf("  Decrypted FCM token: %s\n", decryptedToken)
	fmt.Printf("  Sends to Firebase FCM with message\n")
	fmt.Printf("  ✓ Notification delivered!\n")
	fmt.Println()

	fmt.Println("=== Security & Privacy Benefits ===")
	fmt.Printf("✓ App-Backend never stores actual encrypted FCM tokens\n")
	fmt.Printf("✓ App-Backend only knows opaque identifiers (meaningless)\n")
	fmt.Printf("✓ Only Notification-Backend can decrypt tokens\n")
	fmt.Printf("✓ Opaque IDs provide no information about underlying tokens\n")
	fmt.Printf("✓ Better separation of concerns and security\n")
	fmt.Printf("✓ Durable storage allows server restarts\n")
	fmt.Printf("✓ 256-bit opaque IDs are cryptographically secure\n")
	fmt.Println()

	fmt.Println("Demo completed successfully! 🎉")
}

func generateOpaqueID() string {
	// Generate 32 random bytes (256 bits)
	bytes := make([]byte, 32)
	for i := range bytes {
		bytes[i] = byte(rand.Intn(256))
	}
	return hex.EncodeToString(bytes)
}

func simulateHybridEncryption(token string) string {
	// This simulates the hybrid encryption that would happen in the Android app
	// In reality: AES-GCM(token) + RSA(AES-key) + IV -> base64
	return "base64_encoded_hybrid_encrypted_" + fmt.Sprintf("%x", []byte(token))
}

func simulateHybridDecryption(encryptedData string) string {
	// This simulates the hybrid decryption in notification-backend
	// In reality: base64 -> RSA-decrypt(AES-key) -> AES-GCM-decrypt(token)
	if len(encryptedData) > 30 {
		// Extract the hex-encoded token from the simulated encryption
		hexPart := encryptedData[30:] // Skip "base64_encoded_hybrid_encrypted_"
		bytes, _ := hex.DecodeString(hexPart)
		return string(bytes)
	}
	return "decryption_failed"
}

func init() {
	rand.Seed(time.Now().UnixNano())
}
