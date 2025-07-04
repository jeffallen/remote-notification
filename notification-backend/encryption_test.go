package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"strings"
	"testing"
)

// Test helper to generate a test RSA key pair
func generateTestRSAKeyPair(t *testing.T) (*rsa.PrivateKey, *rsa.PublicKey) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate RSA key pair: %v", err)
	}
	return privKey, &privKey.PublicKey
}

// Test helper to encrypt a token using the same hybrid encryption as Android
func encryptTokenHybrid(token string, publicKey *rsa.PublicKey) (string, error) {
	// Step 1: Generate random AES-256 key
	aesKey := make([]byte, 32)
	if _, err := rand.Read(aesKey); err != nil {
		return "", err
	}

	// Step 2: Generate random IV for GCM
	iv := make([]byte, 12)
	if _, err := rand.Read(iv); err != nil {
		return "", err
	}

	// Step 3: Encrypt token with AES-GCM
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	encryptedToken := gcm.Seal(nil, iv, []byte(token), nil)

	// Step 4: Encrypt AES key with RSA
	encryptedAESKey, err := rsa.EncryptPKCS1v15(rand.Reader, publicKey, aesKey)
	if err != nil {
		return "", err
	}

	// Step 5: Combine all parts: IV + key_length + encrypted_AES_key + encrypted_token
	keyLength := len(encryptedAESKey)
	keyLengthBytes := []byte{
		byte(keyLength >> 24),
		byte(keyLength >> 16),
		byte(keyLength >> 8),
		byte(keyLength),
	}

	combined := make([]byte, 0, 12+4+keyLength+len(encryptedToken))
	combined = append(combined, iv...)
	combined = append(combined, keyLengthBytes...)
	combined = append(combined, encryptedAESKey...)
	combined = append(combined, encryptedToken...)

	return base64.StdEncoding.EncodeToString(combined), nil
}

// Test basic encryption/decryption round-trip
func TestHybridEncryptionRoundTrip(t *testing.T) {
	// Generate test key pair
	privKey, pubKey := generateTestRSAKeyPair(t)

	// Set global private key for decryption function
	originalPrivateKey := privateKey
	privateKey = privKey
	defer func() { privateKey = originalPrivateKey }()

	testTokens := []string{
		"simple_token",
		"token_with_special_chars_!@#$%^&*()",
		"very_long_token_" + strings.Repeat("x", 1000),
		"", // empty token
	}

	for _, token := range testTokens {
		t.Run("Token_"+token[:min(len(token), 20)], func(t *testing.T) {
			// Encrypt
			encrypted, err := encryptTokenHybrid(token, pubKey)
			if err != nil {
				t.Fatalf("Encryption failed: %v", err)
			}

			// Decrypt
			decrypted, err := decryptHybridToken(encrypted)
			if err != nil {
				t.Fatalf("Decryption failed: %v", err)
			}

			// Verify
			if decrypted != token {
				t.Errorf("Round-trip failed: expected %q, got %q", token, decrypted)
			}
		})
	}
}

// Test AEAD corruption detection
func TestAEADCorruptionDetection(t *testing.T) {
	// Generate test key pair
	privKey, pubKey := generateTestRSAKeyPair(t)

	// Set global private key for decryption function
	originalPrivateKey := privateKey
	privateKey = privKey
	defer func() { privateKey = originalPrivateKey }()

	testToken := "test_token_for_corruption"

	// Encrypt token
	encrypted, err := encryptTokenHybrid(testToken, pubKey)
	if err != nil {
		t.Fatalf("Encryption failed: %v", err)
	}

	// Verify original decryption works
	decrypted, err := decryptHybridToken(encrypted)
	if err != nil {
		t.Fatalf("Original decryption failed: %v", err)
	}
	if decrypted != testToken {
		t.Fatalf("Original decryption incorrect: expected %q, got %q", testToken, decrypted)
	}

	// Test corruption at different positions
	corruptionTests := []struct {
		name     string
		position int
	}{
		{"Corrupt IV", 5},
		{"Corrupt key length", 14},
		{"Corrupt encrypted AES key", 20},
		{"Corrupt encrypted token (start)", 300},
		{"Corrupt encrypted token (end)", -5}, // 5 bytes from end
	}

	for _, tc := range corruptionTests {
		t.Run(tc.name, func(t *testing.T) {
			// Decode encrypted data
			data, err := base64.StdEncoding.DecodeString(encrypted)
			if err != nil {
				t.Fatalf("Failed to decode encrypted data: %v", err)
			}

			// Calculate corruption position
			pos := tc.position
			if pos < 0 {
				pos = len(data) + pos
			}

			// Skip if position is out of bounds
			if pos < 0 || pos >= len(data) {
				t.Skipf("Position %d out of bounds for data length %d", pos, len(data))
			}

			// Corrupt the data
			corruptedData := make([]byte, len(data))
			copy(corruptedData, data)
			corruptedData[pos] ^= 0xFF // Flip all bits

			// Encode back to base64
			corruptedEncrypted := base64.StdEncoding.EncodeToString(corruptedData)

			// Attempt decryption - should fail
			_, err = decryptHybridToken(corruptedEncrypted)
			if err == nil {
				t.Error("Expected decryption to fail with corrupted data, but it succeeded")
			} else {
				t.Logf("Decryption correctly failed with error: %v", err)
			}
		})
	}
}

// Test wrong private key
func TestWrongPrivateKey(t *testing.T) {
	// Generate two different key pairs
	_, pubKey1 := generateTestRSAKeyPair(t)
	privKey2, _ := generateTestRSAKeyPair(t)

	testToken := "test_token_for_wrong_key"

	// Encrypt with first public key
	encrypted, err := encryptTokenHybrid(testToken, pubKey1)
	if err != nil {
		t.Fatalf("Encryption failed: %v", err)
	}

	// Try to decrypt with second private key - should fail
	originalPrivateKey := privateKey
	privateKey = privKey2
	defer func() { privateKey = originalPrivateKey }()

	_, err = decryptHybridToken(encrypted)
	if err == nil {
		t.Error("Expected decryption to fail with wrong private key, but it succeeded")
	} else {
		t.Logf("Decryption correctly failed with wrong key: %v", err)
	}
}

// Test malformed encrypted data
func TestMalformedEncryptedData(t *testing.T) {
	// Generate test key pair
	privKey, _ := generateTestRSAKeyPair(t)

	// Set global private key for decryption function
	originalPrivateKey := privateKey
	privateKey = privKey
	defer func() { privateKey = originalPrivateKey }()

	malformedTests := []struct {
		name string
		data string
	}{
		{"Empty string", ""},
		{"Invalid base64", "invalid!!!base64!!!"},
		{"Too short", base64.StdEncoding.EncodeToString([]byte("short"))},
		{"Only IV", base64.StdEncoding.EncodeToString(make([]byte, 12))},
		{"IV + malformed key length", base64.StdEncoding.EncodeToString(append(make([]byte, 12), 0xFF, 0xFF, 0xFF, 0xFF))},
	}

	for _, tc := range malformedTests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := decryptHybridToken(tc.data)
			if err == nil {
				t.Error("Expected decryption to fail with malformed data, but it succeeded")
			} else {
				t.Logf("Decryption correctly failed: %v", err)
			}
		})
	}
}

// Test secure memory wiping functions
func TestSecureMemoryWiping(t *testing.T) {
	// Test string wiping
	testStr := "sensitive_data_to_wipe"
	originalStr := testStr
	secureWipeString(&testStr)

	if testStr != "" {
		t.Errorf("String wiping failed: expected empty string, got %q", testStr)
	}

	// Test byte slice wiping
	testBytes := []byte(originalStr)
	secureWipeBytes(testBytes)

	for i, b := range testBytes {
		if b != 0 {
			t.Errorf("Byte wiping failed at position %d: expected 0, got %d", i, b)
		}
	}
}

// Helper function to get minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
