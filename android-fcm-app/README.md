# FCM Android App

A minimal Android app that registers device tokens with Firebase Cloud Messaging (FCM) and sends them to a server.

## Features

- Single activity with one button to register FCM token
- Sends device token to `https://example.com/register` via HTTP POST
- Displays registration status
- Handles incoming push notifications

## Setup

### 1. Firebase Configuration

1. Go to [Firebase Console](https://console.firebase.google.com/)
2. Create a new project or use an existing one
3. Add an Android app to your project:
   - Package name: `org.nella.fcmapp`
   - App nickname: FCM App (optional)
4. Download the `google-services.json` file
5. Replace the placeholder `google-services.json` in the `app/` directory with the real one

### 2. Build and Run

1. Open the project in Android Studio
2. Sync the project to download dependencies
3. Run the app on a device or emulator
4. Tap "Register Device Token" to get and send the FCM token

## App Structure

- `MainActivity.kt`: Main activity with token registration logic
- `FCMService.kt`: Firebase messaging service for handling incoming notifications
- `activity_main.xml`: Simple UI with title, button, and status text

## API Integration

The app sends a POST request to `https://10.0.2.2:8443/register` with this JSON payload:

Note: `10.0.2.2` is the special IP that Android emulators use to reach the host machine. The app includes certificate bypass logic to ignore self-signed certificate errors.

### Encryption Flow
1. **Token Generation**: Firebase SDK generates device token
2. **RSA Encryption**: Token encrypted with public key (RSA-4096)
3. **Transmission**: Encrypted token sent to app-backend
4. **Pass-through**: App-backend forwards encrypted token without decryption
5. **Decryption**: Notification-backend decrypts token only when needed
6. **Memory Wipe**: Decrypted token immediately removed from memory

```json
{
  "token": "<encrypted-base64-token>",
  "platform": "android",
  "encrypted": true
}
```

The token field contains an RSA-4096 encrypted and base64-encoded device token.

## Dependencies

- Firebase Cloud Messaging: `com.google.firebase:firebase-messaging-ktx:23.3.1`
- OkHttp: `com.squareup.okhttp3:okhttp:4.12.0` (for HTTP requests)
- AndroidX libraries for UI components

## Privacy Considerations

### Token Encryption
This app implements end-to-end encryption for FCM device tokens:

1. **Public Key Encryption**: Device tokens are encrypted using RSA-4096 before transmission
2. **Client-Side Encryption**: Encryption happens on the device using a public key embedded in the app
3. **Zero-Knowledge Intermediate**: The app-backend cannot decrypt tokens, ensuring privacy separation
4. **Decryption at Destination**: Only the notification-backend has the private key to decrypt tokens

### Security Benefits
- **Privacy by Design**: App-backend operators cannot access raw device tokens
- **Organizational Separation**: Different teams can operate app-backend vs notification-backend
- **Compliance**: Easier to demonstrate privacy controls for regulatory requirements
- **Memory Security**: Decrypted tokens are wiped from memory immediately after use

See the chat transcript for additional privacy analysis regarding FCM tokens and law enforcement access.

## Server Side

You'll need to implement a server endpoint at `https://example.com/register` that accepts the token registration. The Go server implementation will be provided separately.
