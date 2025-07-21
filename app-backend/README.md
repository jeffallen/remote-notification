# App Backend

Zero-knowledge intermediary service that relays encrypted FCM tokens without the ability to decrypt them.

## Setup

### 1. Start Notification Backend First

```bash
cd ../notification-backend
go run main.go  # Runs on :8080
```

### 2. Start App Backend

```bash
go run main.go  # Runs on :8081
```

### 3. Configure Android App

Update your Android app to register tokens with:
```
http://localhost:8081/register
```

## API Endpoints

### Register Encrypted Token
```bash
curl -k -X POST https://localhost:8443/register \
  -H "Content-Type: application/json" \
  -d '{"encrypted_data": "<hybrid-encrypted-base64>", "platform": "android"}'
```

Response:
```json
{
  "success": true,
  "message": "Encrypted token registered successfully",
  "platform": "android",
  "total_tokens": 1
}
```

### Send to All Devices
```bash
curl -X POST http://localhost:8081/send-all \
  -d "message=Hello from app backend!"
```

## Web Interface

Visit http://localhost:8081 to:
- View current registered token count
- Send test notifications via web form
- Review privacy design information

## Configuration

Customize with command line flags:

```bash
go run main.go \
  --port=8443 \
  --public-key=public_key.pem \
  --backend-url=http://localhost:8080 \
  --cert=cert.pem \
  --key=key.pem
```

## Privacy Design

- **RAM-Only Storage**: All data lost on restart
- **Zero-Knowledge**: Cannot decrypt tokens even if compromised
- **Pass-Through Architecture**: Forwards encrypted data without processing
- **Organizational Separation**: Different teams can operate each service independently

See the main README for detailed security architecture information.
