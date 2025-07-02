# App Backend - Notification Intermediary

A privacy-focused intermediate server that sits between mobile apps and the notification backend. Designed for organizations that need verifiable separation between device tokens and user data.

## Architecture

```
Mobile App → App Backend (Port 8081) → Notification Backend (Port 8080) → FCM
```

## Privacy Design

- **Encrypted Token Storage**: Only encrypted FCM device tokens stored, no plaintext access
- **Zero-Knowledge**: Cannot decrypt tokens - true privacy separation from user data
- **RAM-Only**: All data lost on restart (no persistent storage)
- **Separate Service**: Runs independently from user data systems
- **Individual Notifications**: Each token sent separately to maintain isolation
- **No User Association**: Tokens never linked to accounts or profiles
- **End-to-End Encryption**: RSA-4096 encryption from client to notification backend

## Features

- **Token Registration**: Accepts tokens from mobile apps and forwards to notification backend
- **Web Interface**: Simple UI showing token count and send functionality
- **Bulk Notifications**: Send messages to all registered devices
- **Privacy Logging**: Safe token truncation in logs

## Setup

### 1. Start Notification Backend

First, make sure the notification-backend is running:

```bash
cd ../notification-backend
go run main.go
# Runs on :8080
```

### 2. Start App Backend

```bash
go run main.go
# Runs on :8081
```

### 3. Configure Mobile App

Update your Android app to register tokens with:
```
http://localhost:8081/register
```

## Usage

### Web Interface

Visit http://localhost:8081 to see:
- Current registered token count
- Notification sending form
- Privacy design information

### API Endpoints

#### Register Token
```bash
curl -k -X POST https://localhost:8443/register \
  -H "Content-Type: application/json" \
  -d '{"token": "<encrypted-base64-token>", "platform": "android", "encrypted": true}'
```

Note: 
- The `-k` flag tells curl to ignore certificate errors
- The token should be RSA-4096 encrypted and base64 encoded
- App-backend cannot decrypt the token - it only stores and forwards it

Response:
```json
{
  "success": true,
  "message": "Token registered successfully",
  "platform": "android",
  "encryption": "encrypted",
  "total_tokens": 1
}
```

#### Send to All (Web Form)
Use the web interface at http://localhost:8081 or:

```bash
curl -X POST http://localhost:8081/send-all \
  -d "message=Hello from app backend!"
```

## Testing Flow

1. Start notification-backend: `cd ../notification-backend && go run main.go`
2. Start app-backend: `go run main.go`
3. Register a token:
   ```bash
   curl -X POST http://localhost:8081/register \
     -H "Content-Type: application/json" \
     -d '{"token": "test-token-123", "platform": "android"}'
   ```
4. Visit http://localhost:8081 to see the token count
5. Send a test notification through the web interface

## Configuration

By default, app-backend forwards to `http://localhost:8080`. To change:

```go
const (
    notificationBackendURL = "http://your-notification-backend:8080"
)
```

## Privacy Benefits

- **Zero-Knowledge Architecture**: App-backend cannot decrypt tokens even if compromised
- **Organizational Separation**: Different teams can operate each service with cryptographic separation
- **Data Minimization**: Only encrypted tokens stored, no plaintext access
- **Audit Trail**: Clear separation makes compliance easier to demonstrate
- **Individual Control**: Each notification sent separately with end-to-end encryption
- **Memory-Only**: No persistent storage of sensitive tokens
- **Forward Secrecy**: Private key never leaves notification-backend environment

### Encryption Architecture

```
Android App ──[RSA-4096]──> App Backend ──[encrypted]──> Notification Backend
   (encrypt)                  (pass-through)              (decrypt + wipe)
```

This design allows the app backend organization to cryptographically prove they cannot access device tokens, providing the strongest possible privacy guarantees.
