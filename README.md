# minnotif-android

## Features

- **End-to-End Encrypted Notifications**: RSA + AES hybrid encryption
- **Opaque Token System**: Privacy-preserving token management
- **Durable Storage**: Exoscale SOS integration for persistent token storage
- **Automatic Cleanup**: Removes unused tokens after 30 days
- **Multiple Storage Backends**: SOS primary, file-based fallback
- **Public Key Hash Namespacing**: Prevents key collision in multi-tenant scenarios
## Security Setup

### Generating New RSA Keypairs for Deployment

For production deployment, generate a new RSA-4096 keypair:

```bash
# Generate private key
openssl genrsa -out private_key.pem 4096

# Generate public key from private key
openssl rsa -in private_key.pem -pubout -out public_key.pem
```

### Files to Update with New Public Key

After generating a new keypair:

1. **Android App**: Replace `demo-app/app/src/main/assets/public_key.pem` with your new public key
2. **Notification Backend**: Place `private_key.pem` in `notification-backend/` directory
3. **Rebuild and Deploy**: The Android app must be rebuilt with the new public key

### Security Notes

- **Never commit** `private_key.pem` to version control
- Keep the private key secure on the notification-backend server only
- Each deployment should use a unique keypair
- Rotate keys periodically for enhanced security

### Hybrid Encryption Architecture

The system uses hybrid encryption for optimal security and performance:

1. **Per-Token AES Key**: Each token gets a unique AES-256 key
2. **RSA Protection**: AES key encrypted with RSA-4096 public key
3. **AEAD Encryption**: Token encrypted with AES-GCM for authenticity
4. **Zero-Knowledge Relay**: App-backend cannot decrypt any data
5. **Memory Security**: Keys wiped immediately after use

```
Android App:
  Token → AES-256-GCM → RSA-4096(AES-key) → Base64 → Network

Notification Backend:
  Base64 → RSA-decrypt(AES-key) → AES-256-GCM-decrypt → Token → FCM
```
