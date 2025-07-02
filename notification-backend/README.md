# FCM Notification Backend

A minimal Go server for receiving FCM token registrations and sending push notifications. Uses only the Go standard library.

## Features

- **Token Registration**: Accept FCM tokens from mobile apps
- **Push Notifications**: Send notifications using Firebase Admin SDK v1 API
- **Simple Storage**: In-memory token storage (tokens lost on restart)
- **Status Monitoring**: Check registered token count and Firebase initialization status
- **Modern API**: Uses Firebase Cloud Messaging API v1 (not legacy API)

## Setup

### 1. Get Firebase Service Account Key

1. Go to [Firebase Console](https://console.firebase.google.com/)
2. Select your project
3. Go to Project Settings > Service Accounts tab
4. Click "Generate new private key"
5. Save the downloaded JSON file as `key.json` in the notification-backend directory

### 2. Configure Server

Ensure the `key.json` file is present in the notification-backend directory. The server will automatically load it on startup.

**Important**: The `key.json` file is gitignored and should never be committed to version control.

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

- **In-Memory Storage**: Tokens are lost when server restarts
- **No Authentication**: This is a minimal demo server
- **Firebase Admin SDK**: Uses official Firebase Admin SDK for Go
- **FCM v1 API**: Uses the modern Firebase Cloud Messaging API v1
- **Service Account Authentication**: Requires Firebase service account key file
- **Automatic Retry**: Firebase SDK handles retry logic and error recovery

## Privacy Considerations

This server stores FCM tokens in memory without linking them to user identities, which aligns with privacy-sensitive design discussed in the chat transcript.
