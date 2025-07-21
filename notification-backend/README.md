# Notification Backend

FCM notification service that decrypts hybrid-encrypted device tokens and sends push notifications.

## Setup

### 1. Firebase Service Account Key

1. Go to [Firebase Console](https://console.firebase.google.com/)
2. Select your project → Project Settings → Service Accounts
3. Click "Generate new private key"
4. Save as `key.json` in this directory

### 2. RSA Private Key

Place the RSA private key as `private_key.pem` in this directory (see main README for key generation).

**Security**: Both `key.json` and `private_key.pem` are gitignored and must never be committed.

### 3. Storage Configuration (Optional)

For production, configure Exoscale SOS:

```bash
go run main.go \
  --sos-access-key=YOUR_ACCESS_KEY \
  --sos-secret-key=YOUR_SECRET_KEY \
  --sos-bucket=notification-tokens \
  --sos-zone=ch-gva-2
```

Without SOS credentials, falls back to local file storage.

### 4. Start Server

```bash
go run main.go  # Runs on :8080
```

## API Endpoints

### Register Encrypted Token
```bash
curl -X POST http://localhost:8080/register \
  -H "Content-Type: application/json" \
  -d '{"encrypted_data": "<hybrid-encrypted-base64>", "platform": "android"}'
```

### Send Notification
```bash
curl -X POST http://localhost:8080/send \
  -H "Content-Type: application/json" \
  -d '{"title": "Hello", "body": "Test notification"}'
```

### Check Status
```bash
curl http://localhost:8080/status
```

Response:
```json
{
  "registered_tokens": 1,
  "firebase_initialized": true,
  "api_version": "FCM v1 (Firebase Admin SDK)"
}
```

## Storage Options

### Exoscale SOS (Recommended)
- Persistent across server restarts
- Automatic 30-day token cleanup
- Public key hash namespacing
- Production scalable

### File Storage (Development)
- Simple setup, no external dependencies
- Limited scalability
- Suitable for testing only

## Security Features

- **Just-in-Time Decryption**: Tokens decrypted only when sending notifications
- **Immediate Memory Wipe**: Decrypted data removed after use
- **Private Key Isolation**: Private key never shared with other components
- **Firebase Admin SDK**: Official SDK with automatic retry logic

See the main README for complete security architecture details.
