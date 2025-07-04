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

### Hybrid Encryption Flow
1. **Token Generation**: Firebase SDK generates device token
2. **AES Key Generation**: Random AES-256 key generated per token
3. **AEAD Encryption**: Token encrypted with AES-GCM (authenticated encryption)
4. **Key Protection**: AES key encrypted with RSA-4096 public key
5. **Data Packaging**: IV + encrypted AES key + encrypted token combined and base64-encoded
6. **Transmission**: Encrypted package sent to app-backend
7. **Zero-Knowledge Relay**: App-backend forwards without ability to decrypt
8. **Hybrid Decryption**: Notification-backend decrypts AES key with RSA, then token with AES
9. **Memory Wipe**: All keys and decrypted data immediately removed from memory

```json
{
  "encrypted_data": "<hybrid-encrypted-base64-data>",
  "platform": "android"
}
```

The `encrypted_data` field contains:
- 12-byte IV for AES-GCM
- 4-byte length of encrypted AES key
- RSA-4096 encrypted AES-256 key
- AES-GCM encrypted FCM token with authentication tag

All combined and base64-encoded for safe transport.

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
- **Hybrid Encryption**: Combines RSA security with AES performance
- **Authenticated Encryption**: AES-GCM provides both confidentiality and authenticity
- **Per-Token Keys**: Each token encrypted with unique AES key
- **Forward Secrecy**: Compromise of one token doesn't affect others
- **Zero-Knowledge Relay**: App-backend cryptographically cannot access tokens
- **Memory Security**: All keys and decrypted data wiped immediately after use
- **No Token Logging**: No plaintext tokens ever logged anywhere in the system

See the chat transcript for additional privacy analysis regarding FCM tokens and law enforcement access.

## Server Side

You'll need to implement a server endpoint at `https://example.com/register` that accepts the token registration. The Go server implementation will be provided separately.
