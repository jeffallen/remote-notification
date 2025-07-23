# Remote Notify Demo App

Android client that registers notification tokens using hybrid encryption and receives push notifications.

## Setup

### 1. Firebase Configuration

1. Go to [Firebase Console](https://console.firebase.google.com/)
2. Create or select a project
3. Add Android app with package name: `org.nella.rn.demo` (demo app)
4. Download `google-services.json` and replace the placeholder in `app/` directory

### 2. Public Key

Ensure `app/src/main/assets/public_key.pem` contains the RSA public key (see main README for key generation).

### 3. Build and Run

1. Open project in Android Studio
2. Sync project to download dependencies
3. Run on device or emulator
4. Tap "Register Device Token" to encrypt and send notification token

## Features

- Single-button notification token registration
- Client-side hybrid encryption (AES-256-GCM + RSA-4096)
- Automatic certificate bypass for development servers
- Push notification handling
- Registration status display

## API Integration

Sends encrypted token to `https://10.0.2.2:8443/register` (emulator) with payload:

```json
{
  "encrypted_data": "<hybrid-encrypted-base64-data>",
  "platform": "android"
}
```

**Note**: `10.0.2.2` is the emulator's host machine IP. For physical devices, update to actual server IP.

## Dependencies

- Firebase Cloud Messaging: `com.google.firebase:firebase-messaging-ktx:24.1.2`
- OkHttp: `com.squareup.okhttp3:okhttp:4.12.0`
- AndroidX UI components

## Testing

1. Start notification-backend and app-backend services
2. Run Android app and register token
3. Send test notification via app-backend web interface or API
4. Verify notification appears on device

See the main README for complete system architecture and security details.
