# Deployment Infrastructure

This document describes the deployment infrastructure created for the minnotif-android project.

## Components

The project consists of three main components:

1. **Demo App** (`demo-app/`) - Android FCM demo app that encrypts and registers FCM tokens
2. **App Backend** (`app-backend/`) - Go server that acts as an intermediate layer, stores encrypted tokens
3. **Notification Backend** (`notification-backend/`) - Go server that decrypts tokens and sends FCM notifications

## Security Architecture

The system implements **hybrid encryption** for maximum security:

- **AES-256-GCM**: Each token encrypted with unique AES key for performance and authenticity
- **RSA-4096**: AES keys encrypted with RSA public key for key exchange
- **AEAD Protection**: GCM mode provides authenticated encryption detecting any tampering
- **Zero-Knowledge Relay**: App-backend cannot decrypt tokens, maintaining privacy separation
- **Memory Security**: Keys are securely wiped from memory after use

## CI/CD Pipeline

### GitHub Workflow (`.github/workflows/ci.yml`)

The CI pipeline includes:

- **Go Server Compilation**: Builds both app-backend and notification-backend
- **Android App Build**: Compiles the Android APK using Gradle
- **Unit Testing**: Runs comprehensive Go tests including encryption validation
- **Security Scanning**: Uses golangci-lint and gosec for code quality and security
- **Parallel Execution**: Optimized for fast feedback

### Test Coverage

#### Encryption Tests (`notification-backend/encryption_test.go`)

Comprehensive test suite proving encryption security:

✅ **Round-trip Encryption**: Validates encrypt→decrypt cycles for various token sizes
✅ **AEAD Corruption Detection**: Proves tampering detection at multiple levels:
  - Corrupted IV → "cipher: message authentication failed"
  - Corrupted key length → "encrypted data malformed"
  - Corrupted RSA key → "crypto/rsa: decryption error"
  - Corrupted token → "cipher: message authentication failed"
✅ **Malformed Data Handling**: Rejects invalid base64, short data, etc.
✅ **Wrong Key Detection**: Fails appropriately with mismatched keys
✅ **Memory Security**: Validates secure memory wiping functions

#### App Backend Tests (`app-backend/handlers_test.go`)

✅ **Token Storage**: Thread-safe in-memory token management
✅ **HTTP Handlers**: Registration and status endpoints
✅ **Concurrency**: Safe multi-threaded token operations

## System Services

### Systemd Service Files

#### App Backend Service (`systemd/app-backend.service`)
```ini
[Unit]
Description=App Backend Server
After=network.target

[Service]
Type=simple
User=app-backend
WorkingDirectory=/var/lib/app-backend
ExecStart=/usr/bin/app-backend
Restart=always

# Security hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
```

#### Notification Backend Service (`systemd/notification-backend.service`)
```ini
[Unit]
Description=Notification Backend Server
After=network.target

[Service]
Type=simple
User=notification-backend
WorkingDirectory=/var/lib/notification-backend
ExecStart=/usr/bin/notification-backend
Restart=always

# Security hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
```

## Installation

### Automated Installation (`Makefile`)

```bash
# Build all components
make build

# Run tests
make test

# Install Go servers to /usr/bin (requires sudo)
make install

# Build Android app
make android
```

The `make install` target:
1. Creates dedicated system users for each service
2. Sets up secure working directories (`/var/lib/{service}`)
3. Installs binaries to `/usr/bin/`
4. Installs systemd service files
5. Reloads systemd configuration

### Manual Service Management

```bash
# Enable and start services
sudo systemctl enable app-backend notification-backend
sudo systemctl start app-backend notification-backend

# Check status
sudo systemctl status app-backend
sudo systemctl status notification-backend

# View logs
sudo journalctl -u app-backend -f
sudo journalctl -u notification-backend -f
```

## Security Features

### Encryption Validation

The test suite **proves** that the AEAD encryption correctly:
- Detects **any** corruption in encrypted data
- Rejects tampered tokens with clear error messages
- Maintains data integrity through authenticated encryption
- Securely wipes sensitive data from memory

### System Hardening

- **Dedicated Users**: Each service runs as its own non-privileged user
- **Directory Isolation**: Services can only write to their specific directories
- **No New Privileges**: Prevents privilege escalation
- **Private Temp**: Isolated temporary file access
- **System Protection**: Read-only system directories

### Network Security

- **HTTPS Only**: App-backend uses TLS for client connections
- **Internal Communication**: Backends communicate over localhost
- **Port Separation**: Different ports for each service (8080, 8443)

## Deployment Checklist

- [ ] Generate production RSA keypair (see README.md)
- [ ] Configure Firebase service account key
- [ ] Update Android app with production public key
- [ ] Build and deploy Go servers: `make install`
- [ ] Enable systemd services
- [ ] Configure firewall rules
- [ ] Set up monitoring and log rotation
- [ ] Test end-to-end functionality

## Monitoring

- **Systemd Logs**: Centralized logging via journald
- **Service Status**: Built-in systemd health monitoring
- **Process Management**: Automatic restart on failure
- **Resource Limits**: Configured file and process limits

## Development

```bash
# Quick development build
make build

# Run specific tests
cd notification-backend && go test -run TestAEADCorruptionDetection -v

# Clean build artifacts
make clean
```

This infrastructure provides production-ready deployment with comprehensive security testing, proving that the hybrid encryption system correctly protects user data and detects any tampering attempts.
