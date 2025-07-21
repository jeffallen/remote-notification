# Exoscale SOS Storage Setup

This document describes how to configure the notification backend to use Exoscale Simple Object Storage (SOS) for durable token storage.

## Overview

The notification backend now supports durable storage using Exoscale SOS, which provides:

- **Persistent Storage**: Tokens survive server restarts
- **Scalable Architecture**: Can handle large numbers of tokens
- **Automatic Cleanup**: Old tokens are automatically removed after 30 days of inactivity
- **Security**: Uses public key hash in storage keys for enhanced privacy
- **Geographic Distribution**: Supports multiple Exoscale zones

## Storage Key Structure

Tokens are stored using the key format:
```
public-key-hash/opaque-token-id
```

Example:
```
d65c586e037193d2fb27d01ff123872cbbabe7e8696ec07f51936c9794c75c39/a1b2c3d4e5f6789...
```

## Configuration

### Exoscale SOS Credentials

You'll need:
1. **Access Key**: Your Exoscale SOS access key
2. **Secret Key**: Your Exoscale SOS secret key
3. **Zone**: The Exoscale zone (default: `ch-gva-2`)
4. **Bucket Name**: S3 bucket for token storage (default: `notification-tokens`)

### Notification Backend Configuration

Start the notification backend with Exoscale SOS credentials:

```bash
./notification-backend \
  --sos-access-key=YOUR_ACCESS_KEY \
  --sos-secret-key=YOUR_SECRET_KEY \
  --sos-bucket=notification-tokens \
  --sos-zone=ch-gva-2 \
  --public-key=public_key.pem \
  --private-key=private_key.pem \
  --firebase-key=key.json
```

### App Backend Configuration

The app backend needs the same public key to compute the correct hash:

```bash
./app-backend \
  --public-key=public_key.pem \
  --cert=cert.pem \
  --key=key.pem \
  --backend-url=http://localhost:8080
```

### Available Exoscale Zones

- `ch-gva-2` (Geneva, Switzerland) - **Default**
- `ch-dk-2` (Zurich, Switzerland)
- `at-vie-1` (Vienna, Austria)
- `de-fra-1` (Frankfurt, Germany)
- `de-muc-1` (Munich, Germany)
- `bg-sof-1` (Sofia, Bulgaria)

## Command Line Options

### Notification Backend

| Flag | Default | Description |
|------|---------|-------------|
| `--sos-access-key` | _(empty)_ | Exoscale SOS access key |
| `--sos-secret-key` | _(empty)_ | Exoscale SOS secret key |
| `--sos-bucket` | `notification-tokens` | SOS bucket name |
| `--sos-zone` | `ch-gva-2` | Exoscale zone |
| `--public-key` | `public_key.pem` | Path to RSA public key |

### App Backend

| Flag | Default | Description |
|------|---------|-------------|
| `--public-key` | `public_key.pem` | Path to RSA public key (must match backend) |

## Automatic Token Cleanup

The notification backend includes a built-in cleanup routine that:

- **Runs every 24 hours** (plus initial cleanup after 5 minutes)
- **Removes tokens unused for 30+ days**
- **Logs cleanup activity** for monitoring
- **Tracks last used time** for each notification sent

## Fallback Mode

If no SOS credentials are provided:

- System falls back to local file storage (`tokens.json`)
- **Warning**: File storage is not recommended for production
- All functionality works, but tokens are lost on restart

## Testing the Setup

1. **Start the notification backend**:
   ```bash
   ./notification-backend --sos-access-key=KEY --sos-secret-key=SECRET
   ```

2. **Verify storage initialization** in the logs:
   ```
   Exoscale SOS storage initialized: bucket=notification-tokens, zone=ch-gva-2
   ```

3. **Check the status endpoint**:
   ```bash
   curl http://localhost:8080/status
   ```
   
   Should return:
   ```json
   {
     "registered_tokens": 0,
     "firebase_initialized": true,
     "storage_type": "Exoscale SOS (bucket: notification-tokens, zone: ch-gva-2)",
     "public_key_hash": "d65c586e037193d2..."
   }
   ```

4. **Register a test token** using your mobile app or the registration endpoint

5. **Verify token storage** by checking the bucket in Exoscale console

## Monitoring

Logs will show:
- Storage initialization and configuration
- Token registration and retrieval
- Cleanup operations and results
- Any storage errors or warnings

## Security Considerations

- **Public Key Hash**: Used as a namespace to prevent key collision
- **Opaque Token IDs**: No sensitive information in storage keys
- **Encrypted Data**: Token payloads remain encrypted at rest
- **Access Controls**: Secure your SOS credentials appropriately
- **Network Security**: Use HTTPS and secure network connections

## Troubleshooting

### Common Issues

1. **"bucket does not exist"**: The bucket will be created automatically if you have permissions

2. **"failed to load SOS configuration"**: Check your access key and secret key

3. **"Public Key Hash mismatch"**: Ensure both backend and app-backend use the same public key file

4. **"Token ID not found"**: May indicate storage connectivity issues or mismatched public key hashes

### Debug Mode

Increase logging verbosity to troubleshoot:
```bash
GO_LOG_LEVEL=debug ./notification-backend --sos-access-key=KEY ...
```

## Migration from File Storage

To migrate existing file-based tokens to SOS:

1. **Keep the old `tokens.json`** as backup
2. **Start with SOS configuration**
3. **Re-register tokens** (recommended for security)
4. **Old file storage** will be used as fallback if SOS is unavailable

---

**Note**: This implementation is compatible with any S3-compatible storage service, not just Exoscale SOS. Adjust the endpoint configuration in `storage.go` for other providers.
