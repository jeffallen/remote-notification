# Usage Guide

This guide explains how to configure and run the minnotif-android components.

## Go Server Configuration

Both Go servers now support command-line arguments for easy configuration:

### App Backend Server

```bash
# Default usage
./app-backend

# Custom configuration
./app-backend \
  -port 8443 \
  -cert /path/to/cert.pem \
  -key /path/to/key.pem \
  -backend-url http://localhost:8080
```

**Available Options:**
- `-port`: Port to listen on (default: `8443`)
- `-cert`: Path to TLS certificate file (default: `cert.pem`)
- `-key`: Path to TLS private key file (default: `key.pem`)
- `-backend-url`: URL of the notification backend service (default: `http://localhost:8080`)

### Notification Backend Server

```bash
# Default usage
./notification-backend

# Custom configuration
./notification-backend \
  -port 8080 \
  -firebase-key /path/to/service-account.json \
  -private-key /path/to/private_key.pem
```

**Available Options:**
- `-port`: Port to listen on (default: `8080`)
- `-firebase-key`: Path to Firebase service account key file (default: `key.json`)
- `-private-key`: Path to RSA private key file (default: `private_key.pem`)

## Android App Configuration

The Android app now includes a settings page for configuring the backend URL:

### Accessing Settings

1. **Open the app**
2. **Tap the settings icon** in the top-right menu
3. **Configure backend URL**

### Backend URL Configuration

- **Default URL**: `https://r-notify.nella.org`
- **Custom URLs**: Any HTTPS URL pointing to your app-backend server
- **Local Development**: `https://10.0.2.2:8443` (Android emulator)

### URL Examples

```
# Production server
https://r-notify.nella.org

# Custom domain
https://notifications.yoursite.com

# Local development (Android emulator)
https://10.0.2.2:8443

# Local development (physical device)
https://192.168.1.100:8443
```

## Deployment Examples

### Production Deployment

```bash
# Start notification backend
./notification-backend \
  -port 8080 \
  -firebase-key /etc/minnotif/firebase-key.json \
  -private-key /etc/minnotif/private_key.pem

# Start app backend  
./app-backend \
  -port 443 \
  -cert /etc/ssl/certs/your-cert.pem \
  -key /etc/ssl/private/your-key.pem \
  -backend-url http://localhost:8080
```

### Development Setup

```bash
# Terminal 1: Start notification backend
./notification-backend -port 8080

# Terminal 2: Start app backend
./app-backend -port 8443 -backend-url http://localhost:8080
```

### Docker Deployment

```dockerfile
# Dockerfile.app-backend
FROM golang:alpine AS builder
COPY . /app
WORKDIR /app/app-backend
RUN go build -o app-backend .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/app-backend/app-backend /usr/bin/
EXPOSE 8443
CMD ["app-backend", "-port", "8443"]
```

```bash
# Run with Docker
docker run -p 8443:8443 \
  -v /path/to/certs:/certs \
  your-app-backend \
  -cert /certs/cert.pem \
  -key /certs/key.pem
```

## Systemd Service Configuration

Update the systemd service files to use custom configuration:

### App Backend Service

```ini
# /etc/systemd/system/app-backend.service
[Unit]
Description=App Backend Server
After=network.target

[Service]
Type=simple
User=app-backend
WorkingDirectory=/var/lib/app-backend
ExecStart=/usr/bin/app-backend \
  -port 8443 \
  -cert /etc/ssl/certs/app-backend.pem \
  -key /etc/ssl/private/app-backend.key \
  -backend-url http://localhost:8080
Restart=always

[Install]
WantedBy=multi-user.target
```

### Notification Backend Service

```ini
# /etc/systemd/system/notification-backend.service
[Unit]
Description=Notification Backend Server
After=network.target

[Service]
Type=simple
User=notification-backend
WorkingDirectory=/var/lib/notification-backend
ExecStart=/usr/bin/notification-backend \
  -port 8080 \
  -firebase-key /var/lib/notification-backend/firebase-key.json \
  -private-key /var/lib/notification-backend/private_key.pem
Restart=always

[Install]
WantedBy=multi-user.target
```

## Environment Variables

For containerized deployments, you can also use environment variables:

```bash
# App Backend
export APP_BACKEND_PORT=8443
export APP_BACKEND_CERT=/path/to/cert.pem
export APP_BACKEND_KEY=/path/to/key.pem
export APP_BACKEND_URL=http://localhost:8080

# Notification Backend
export NOTIFICATION_PORT=8080
export FIREBASE_KEY_PATH=/path/to/firebase-key.json
export PRIVATE_KEY_PATH=/path/to/private_key.pem
```

## Security Considerations

### TLS Configuration

- **Always use HTTPS** for the app-backend in production
- **Generate strong certificates** with proper CN/SAN configuration
- **Update Android network security config** if using custom CAs

### Firewall Configuration

```bash
# Allow app-backend (HTTPS)
sudo ufw allow 8443/tcp

# Block notification-backend from external access
sudo ufw deny 8080/tcp
```

### File Permissions

```bash
# Secure private key files
sudo chmod 600 /etc/ssl/private/app-backend.key
sudo chmod 600 /var/lib/notification-backend/private_key.pem
sudo chmod 600 /var/lib/notification-backend/firebase-key.json

# Set ownership
sudo chown app-backend:app-backend /etc/ssl/private/app-backend.key
sudo chown notification-backend:notification-backend /var/lib/notification-backend/*
```

## Monitoring and Logging

### Log Output

Both servers provide structured logging:

```bash
# View logs
sudo journalctl -u app-backend -f
sudo journalctl -u notification-backend -f

# Check configuration
sudo journalctl -u app-backend --since "1 hour ago" | grep "Configuration:"
```

### Health Checks

```bash
# Check app-backend status
curl -k https://localhost:8443/

# Check notification-backend status
curl http://localhost:8080/status
```

## Troubleshooting

### Common Issues

**"Permission denied" on startup:**
```bash
# Check file permissions
ls -la /path/to/cert.pem /path/to/key.pem
sudo chown app-backend:app-backend /path/to/cert.pem
```

**"Connection refused" from Android app:**
```bash
# Check if service is running
sudo systemctl status app-backend

# Check firewall
sudo ufw status

# Verify URL in Android settings
```

**"Firebase initialization failed:**
```bash
# Verify service account key
./notification-backend -firebase-key /path/to/key.json

# Check file permissions
ls -la /path/to/firebase-key.json
```

This flexible configuration system allows easy deployment across different environments while maintaining security and proper separation of concerns.
