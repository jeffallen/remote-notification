package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

)

// TokenStorageInfo represents the data stored for each token
type TokenStorageInfo struct {
	OpaqueID        string    `json:"opaque_id"`
	EncryptedData   string    `json:"encrypted_data"`
	Platform        string    `json:"platform"`
	RegisteredAt    time.Time `json:"registered_at"`
	LastUsedAt      time.Time `json:"last_used_at"`
	PublicKeyHash   string    `json:"public_key_hash"`
}

// ExoscaleStorage provides S3-compatible storage using Exoscale SOS
type ExoscaleStorage struct {
	client       *s3.Client
	bucketName   string
	publicKeyHash string
}

// NewExoscaleStorage creates a new storage instance configured for Exoscale SOS
func NewExoscaleStorage(accessKey, secretKey, bucketName, zone, publicKeyHash string) (*ExoscaleStorage, error) {
	// Configure AWS SDK for Exoscale SOS
	sosCfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
		config.WithRegion(zone),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load SOS configuration: %v", err)
	}

	// Create S3 client with custom endpoint for Exoscale SOS
	sosEndpoint := fmt.Sprintf("https://sos-%s.exo.io", zone)
	client := s3.NewFromConfig(sosCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(sosEndpoint)
		o.UsePathStyle = true // Required for Exoscale SOS
	})

	storage := &ExoscaleStorage{
		client:        client,
		bucketName:    bucketName,
		publicKeyHash: publicKeyHash,
	}

	// Verify bucket exists and is accessible
	if err := storage.ensureBucket(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ensure bucket exists: %v", err)
	}

	log.Printf("Exoscale SOS storage initialized: bucket=%s, zone=%s, endpoint=%s", bucketName, zone, sosEndpoint)
	return storage, nil
}

// ensureBucket checks if the bucket exists and creates it if necessary
func (s *ExoscaleStorage) ensureBucket(ctx context.Context) error {
	// Check if bucket exists
	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(s.bucketName),
	})

	if err != nil {
		// Try to create the bucket
		_, createErr := s.client.CreateBucket(ctx, &s3.CreateBucketInput{
			Bucket: aws.String(s.bucketName),
		})
		if createErr != nil {
			return fmt.Errorf("bucket does not exist and cannot be created: %v (original error: %v)", createErr, err)
		}
		log.Printf("Created new SOS bucket: %s", s.bucketName)
	}

	return nil
}

// StoreToken stores a token in SOS with the key format: public-key-hash/opaque-token-id
func (s *ExoscaleStorage) StoreToken(ctx context.Context, opaqueID, encryptedData, platform string) error {
	info := TokenStorageInfo{
		OpaqueID:      opaqueID,
		EncryptedData: encryptedData,
		Platform:      platform,
		RegisteredAt:  time.Now(),
		LastUsedAt:    time.Now(),
		PublicKeyHash: s.publicKeyHash,
	}

	data, err := json.Marshal(info)
	if err != nil {
		return fmt.Errorf("failed to marshal token info: %v", err)
	}

	key := s.buildObjectKey(opaqueID)
	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucketName),
		Key:         aws.String(key),
		Body:        strings.NewReader(string(data)),
		ContentType: aws.String("application/json"),
	})

	if err != nil {
		return fmt.Errorf("failed to store token in SOS: %v", err)
	}

	log.Printf("Token stored in SOS: %s (key: %s)", opaqueID[:16]+"...", key)
	return nil
}

// GetToken retrieves a token from SOS and updates its last used time
func (s *ExoscaleStorage) GetToken(ctx context.Context, opaqueID string) (*TokenStorageInfo, error) {
	key := s.buildObjectKey(opaqueID)
	resp, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(key),
	})

	if err != nil {
		return nil, fmt.Errorf("failed to get token from SOS: %v", err)
	}
	defer resp.Body.Close()

	var info TokenStorageInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("failed to decode token info: %v", err)
	}

	// Update last used time
	info.LastUsedAt = time.Now()
	if err := s.updateLastUsed(ctx, opaqueID, &info); err != nil {
		log.Printf("Warning: failed to update last used time for %s: %v", opaqueID[:16]+"...", err)
		// Don't fail the get operation if we can't update the timestamp
	}

	return &info, nil
}

// updateLastUsed updates the last used timestamp for a token
func (s *ExoscaleStorage) updateLastUsed(ctx context.Context, opaqueID string, info *TokenStorageInfo) error {
	data, err := json.Marshal(info)
	if err != nil {
		return fmt.Errorf("failed to marshal updated token info: %v", err)
	}

	key := s.buildObjectKey(opaqueID)
	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucketName),
		Key:         aws.String(key),
		Body:        strings.NewReader(string(data)),
		ContentType: aws.String("application/json"),
	})

	return err
}

// ListAllTokens returns all tokens (used for broadcast and cleanup)
func (s *ExoscaleStorage) ListAllTokens(ctx context.Context) ([]*TokenStorageInfo, error) {
	prefix := s.publicKeyHash + "/"
	resp, err := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucketName),
		Prefix: aws.String(prefix),
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list objects: %v", err)
	}

	var tokens []*TokenStorageInfo
	for _, obj := range resp.Contents {
		// Get each object
		getResp, err := s.client.GetObject(ctx, &s3.GetObjectInput{
			Bucket: aws.String(s.bucketName),
			Key:    obj.Key,
		})
		if err != nil {
			log.Printf("Warning: failed to get object %s: %v", *obj.Key, err)
			continue
		}

		var info TokenStorageInfo
		if err := json.NewDecoder(getResp.Body).Decode(&info); err != nil {
			log.Printf("Warning: failed to decode object %s: %v", *obj.Key, err)
			getResp.Body.Close()
			continue
		}
		getResp.Body.Close()

		tokens = append(tokens, &info)
	}

	return tokens, nil
}

// DeleteToken removes a token from storage
func (s *ExoscaleStorage) DeleteToken(ctx context.Context, opaqueID string) error {
	key := s.buildObjectKey(opaqueID)
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(key),
	})

	if err != nil {
		return fmt.Errorf("failed to delete token from SOS: %v", err)
	}

	log.Printf("Token deleted from SOS: %s", opaqueID[:16]+"...")
	return nil
}

// CleanupOldTokens removes tokens that haven't been used in the specified duration
func (s *ExoscaleStorage) CleanupOldTokens(ctx context.Context, maxAge time.Duration) (int, error) {
	tokens, err := s.ListAllTokens(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to list tokens for cleanup: %v", err)
	}

	cutoff := time.Now().Add(-maxAge)
	deleted := 0

	for _, token := range tokens {
		if token.LastUsedAt.Before(cutoff) {
			if err := s.DeleteToken(ctx, token.OpaqueID); err != nil {
				log.Printf("Warning: failed to delete old token %s: %v", token.OpaqueID[:16]+"...", err)
				continue
			}
			deleted++
			log.Printf("Cleaned up token %s (last used: %s)", token.OpaqueID[:16]+"...", token.LastUsedAt.Format("2006-01-02 15:04:05"))
		}
	}

	log.Printf("Cleanup completed: deleted %d tokens older than %v", deleted, maxAge)
	return deleted, nil
}

// buildObjectKey constructs the S3 object key in the format: public-key-hash/opaque-token-id
func (s *ExoscaleStorage) buildObjectKey(opaqueID string) string {
	return fmt.Sprintf("%s/%s", s.publicKeyHash, opaqueID)
}

// ComputePublicKeyHash computes a SHA256 hash of the public key for use in storage keys
func ComputePublicKeyHash(publicKeyPEM string) string {
	hash := sha256.Sum256([]byte(publicKeyPEM))
	return hex.EncodeToString(hash[:])
}
