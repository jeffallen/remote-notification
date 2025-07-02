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

The app sends a POST request to `https://example.com/register` with this JSON payload:

```json
{
  "token": "fcm-device-token-here",
  "platform": "android"
}
```

## Dependencies

- Firebase Cloud Messaging: `com.google.firebase:firebase-messaging-ktx:23.3.1`
- OkHttp: `com.squareup.okhttp3:okhttp:4.12.0` (for HTTP requests)
- AndroidX libraries for UI components

## Privacy Considerations

This app demonstrates minimal FCM integration. The FCM token itself doesn't contain personal information, but Google can potentially link tokens to user accounts when compelled by legal process. See the chat transcript for detailed privacy analysis.

## Server Side

You'll need to implement a server endpoint at `https://example.com/register` that accepts the token registration. The Go server implementation will be provided separately.
