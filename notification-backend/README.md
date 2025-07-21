# FCM Notification Backend

A minimal Go server for receiving FCM token registrations and sending push notifications. Uses only the Go standard library.

## Features

- **Hybrid Encrypted Tokens**: Accept AES-GCM + RSA encrypted FCM tokens from app-backend
- **Push Notifications**: Send notifications using Firebase Admin SDK v1 API
- **Hybrid Decryption**: Decrypt AES keys with RSA, then tokens with AES-GCM
- **Durable Storage**: Exoscale SOS cloud storage with automatic cleanup (file fallback available)
- **Opaque Token System**: Privacy-preserving token management with public key hash namespacing
- **Status Monitoring**: Check registered token count and Firebase initialization status
- **Modern API**: Uses Firebase Cloud Messaging API v1 (not legacy API)
- **Privacy by Design**: Private key isolation and secure memory handling

## Setup

### 1. Get Firebase Service Account Key

1. Go to [Firebase Console](https://console.firebase.google.com/)
2. Select your project
3. Go to Project Settings > Service Accounts tab
4. Click "Generate new private key"
5. Save the downloaded JSON file as `key.json` in the notification-backend directory

### 2. Set up RSA Private Key

The notification-backend requires the RSA private key to decrypt device tokens:

1. The private key (`private_key.pem`) should be placed in the notification-backend directory
2. This key is automatically generated during setup and corresponds to the public key in the Android app
3. **Critical**: Never commit the private key to version control

**Key Security**:
- Private key stays only on notification-backend server
- App-backend never has access to private key
- Tokens are decrypted only when needed and immediately wiped from memory

### 3. Configure Server

Ensure both required files are present in the notification-backend directory:
- `key.json` - Firebase service account key
- `private_key.pem` - RSA private key for token decryption

The server will automatically load both on startup and extract the project ID from the service account key.

**Important**: Both files are gitignored and should never be committed to version control.

### 3. Configure Storage (Optional)

For production deployments, configure Exoscale SOS for durable token storage:

```bash
# Start with Exoscale SOS storage
go run main.go \
  --sos-access-key=YOUR_ACCESS_KEY \
  --sos-secret-key=YOUR_SECRET_KEY \
  --sos-bucket=notification-tokens \
  --sos-zone=ch-gva-2
```

Without SOS credentials, the server falls back to local file storage.

### 4. Run Server

```bash
go run main.go
```

Server starts on `http://localhost:8080`

## API Endpoints

### Register Token
```bash
curl -X POST http://localhost:8080/register \
  -H "Content-Type: application/json" \
  -d '{"encrypted_data": "<hybrid-encrypted-base64>", "platform": "android"}'
```

Note: The `encrypted_data` contains hybrid-encrypted token (AES-GCM with RSA-protected key). The server stores encrypted data and performs hybrid decryption only when sending notifications.

Response:
```json
{
  "success": true,
  "message": "Token registered successfully",
  "platform": "android",
  "total_tokens": 1
}
```

### Send Notification
```bash
curl -X POST http://localhost:8080/send \
  -H "Content-Type: application/json" \
  -d '{"title": "Hello", "body": "Test notification from server"}'
```

Response:
```json
{
  "success": true,
  "message": "Sent to 1 devices, 0 failures",
  "sent_count": 1,
  "error_count": 0,
  "total_tokens": 1
}
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

### Help
```bash
curl http://localhost:8080/
```

Shows available endpoints and current status.

## Testing with Android App

1. Start the server: `go run main.go`
2. Run the Android app and tap "Register Device Token"
3. Check registration: `curl http://localhost:8080/status`
4. Send test notification: 
   ```bash
   curl -X POST http://localhost:8080/send \
     -H "Content-Type: application/json" \
     -d '{"title": "Test", "body": "Hello from server!"}'
   ```

## Notes

- **Durable Storage**: Tokens stored in Exoscale SOS with automatic 30-day cleanup
- **Fallback Mode**: File-based storage available when SOS credentials not provided
- **No Authentication**: This is a minimal demo server
- **Firebase Admin SDK**: Uses official Firebase Admin SDK for Go
- **FCM v1 API**: Uses the modern Firebase Cloud Messaging API v1
- **Service Account Authentication**: Requires Firebase service account key file
- **RSA Decryption**: Tokens decrypted only when needed, immediately wiped from memory
- **Private Key Security**: Private key never shared, stays on notification-backend only
- **Automatic Retry**: Firebase SDK handles retry logic and error recovery

## Encryption Architecture

```
Android App ──[Hybrid Encrypt]──> App Backend ──[Pass Through]──> Notification Backend
                                                                         │
                                                                         v
                                                               [Hybrid Decrypt + Send + Wipe]
                                                                         │
                                                                         v
                                                                    Firebase FCM
```

### Security Features

- **Zero-Knowledge Intermediate**: App-backend cannot decrypt tokens
- **Just-in-Time Decryption**: Tokens decrypted only when sending notifications
- **Immediate Memory Wipe**: Decrypted tokens removed from memory after use
- **Private Key Isolation**: Private key never leaves notification-backend environment
- **Hybrid Encryption**: AES-GCM for performance, RSA for key protection
- **AEAD Security**: Authenticated encryption prevents tampering
- **Per-Token Keys**: Each token encrypted with unique AES key

## Privacy Considerations

This server stores FCM tokens in memory without linking them to user identities, which aligns with privacy-sensitive design discussed in the chat transcript.

## Storage Options

### Exoscale SOS (Recommended)

- **Persistent**: Tokens survive server restarts
- **Scalable**: Handles large numbers of devices
- **Auto-cleanup**: Removes unused tokens after 30 days
- **Secure**: Public key hash namespacing prevents collisions

### File Storage (Fallback)

- **Simple**: No external dependencies
- **Limited**: Not suitable for production scale
- **Temporary**: Tokens persist across restarts but not recommended for production

See `EXOSCALE_SOS_SETUP.md` for detailed configuration instructions.
