# Bold

Privacy-focused notification system with end-to-end encrypted FCM token management.

## Architecture

```
Android App → App Backend (8081) → Notification Backend (8080) → Firebase FCM
```

**Privacy by Design**: The app-backend acts as a zero-knowledge intermediary that cannot decrypt device tokens, ensuring organizational separation between user data and notification infrastructure.

## Features

- **End-to-End Encrypted Notifications**: RSA + AES hybrid encryption
- **Zero-Knowledge Intermediary**: App-backend cannot decrypt tokens
- **Durable Storage**: Exoscale SOS integration with automatic cleanup
- **Public Key Hash Namespacing**: Prevents key collision in multi-tenant scenarios
- **Modern FCM API**: Uses Firebase Cloud Messaging API v1

## Quick Start

### 1. Generate RSA Keypair

```bash
# Generate private key
openssl genrsa -out private_key.pem 4096

# Generate public key from private key
openssl rsa -in private_key.pem -pubout -out public_key.pem
```

### 2. Deploy Keys

- **Android App**: Replace `demo-app/app/src/main/assets/public_key.pem`
- **Notification Backend**: Place `private_key.pem` in `notification-backend/`
- **Never commit** `private_key.pem` to version control

### 3. Start Services

```bash
# Terminal 1: Start notification backend
cd notification-backend
go run main.go  # Runs on :8080

# Terminal 2: Start app backend  
cd app-backend
go run main.go  # Runs on :8081
```

### 4. Configure Firebase

See `notification-backend/README.md` for Firebase setup instructions.

## Security Architecture

### Hybrid Encryption Flow

1. **Android App**: Token → AES-256-GCM → RSA-4096(AES-key) → Base64 → Network
2. **App Backend**: Pass-through (zero-knowledge)
3. **Notification Backend**: Base64 → RSA-decrypt(AES-key) → AES-GCM-decrypt → Token → FCM

### Privacy Guarantees

- **Zero-Knowledge Relay**: App-backend cryptographically cannot access tokens
- **Just-in-Time Decryption**: Tokens decrypted only when sending notifications
- **Memory Security**: Keys wiped immediately after use
- **Private Key Isolation**: Private key never leaves notification-backend
- **Per-Token Keys**: Each token encrypted with unique AES key

## Components

- **[demo-app](demo-app/)**: Android FCM client with hybrid encryption
- **[app-backend](app-backend/)**: Zero-knowledge intermediary service
- **[notification-backend](notification-backend/)**: FCM notification service with token decryption

See individual component READMEs for detailed setup and API documentation.
