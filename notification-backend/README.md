# FCM Notification Backend

A minimal Go server for receiving FCM token registrations and sending push notifications. Uses only the Go standard library.

## Features

- **Token Registration**: Accept FCM tokens from mobile apps
- **Push Notifications**: Send notifications to all registered devices
- **Simple Storage**: In-memory token storage (tokens lost on restart)
- **Status Monitoring**: Check registered token count and server config

## Setup

### 1. Get FCM Server Key

1. Go to [Firebase Console](https://console.firebase.google.com/)
2. Select your project
3. Go to Project Settings > Cloud Messaging tab
4. Copy the "Server key" (Legacy server key)

### 2. Configure Server

Edit `main.go` and replace:
```go
serverKey = "YOUR_FCM_SERVER_KEY_HERE"
```

With your actual FCM server key:
```go
serverKey = "AAAAxxx...:APA91bH..."
```

### 3. Run Server

```bash
go run main.go
```

Server starts on `http://localhost:8080`

## API Endpoints

### Register Token
```bash
curl -X POST http://localhost:8080/register \
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
  "server_key_configured": true
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

- **In-Memory Storage**: Tokens are lost when server restarts
- **No Authentication**: This is a minimal demo server
- **Single-threaded**: Uses Go's default HTTP server (which is actually concurrent)
- **Standard Library Only**: No external dependencies
- **Legacy FCM API**: Uses the simpler legacy API instead of HTTP v1

## Privacy Considerations

This server stores FCM tokens in memory without linking them to user identities, which aligns with privacy-sensitive design discussed in the chat transcript.
