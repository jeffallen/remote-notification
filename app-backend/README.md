# App Backend - Notification Intermediary

A privacy-focused intermediate server that sits between mobile apps and the notification backend. Designed for organizations that need verifiable separation between device tokens and user data.

## Architecture

```
Mobile App → App Backend (Port 8081) → Notification Backend (Port 8080) → FCM
```

## Privacy Design

- **Token-Only Storage**: Only FCM device tokens stored, no user data
- **RAM-Only**: All data lost on restart (no persistent storage)
- **Separate Service**: Runs independently from user data systems
- **Individual Notifications**: Each token sent separately to maintain isolation
- **No User Association**: Tokens never linked to accounts or profiles

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
curl -X POST http://localhost:8081/register \
  -H "Content-Type: application/json" \
  -d '{"token": "fcm-device-token", "platform": "android"}'
```

Response:
```json
{
  "success": true,
  "message": "Token registered successfully",
  "platform": "android",
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

- **Organizational Separation**: Different teams can operate each service
- **Data Minimization**: Only tokens stored, no user context
- **Audit Trail**: Clear separation makes compliance easier
- **Individual Control**: Each notification sent separately
- **Memory-Only**: No persistent storage of sensitive tokens

This design allows the app backend organization to verifiably demonstrate that they handle only device tokens and cannot associate them with user identities or behaviors.
