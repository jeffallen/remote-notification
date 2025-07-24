# Certificate Security Configuration

This document explains the certificate validation configuration for development and production builds.

## Overview

The notification system uses different certificate validation strategies depending on the build type:

- **Debug builds**: Allow self-signed certificates for development
- **Release builds**: Strict certificate validation using system CAs only

## Build-Specific Configuration

### Debug Builds (Development)

**File**: `demo-app/app/src/debug/res/xml/network_security_config.xml`

- Allows self-signed certificates for localhost, 127.0.0.1, and 10.0.2.2 (Android emulator)
- Trusts both system CAs and user-added certificates
- Includes debug overrides for development flexibility
- **WARNING**: Never use debug configuration in production

### Release Builds (Production)

**File**: `demo-app/app/src/release/res/xml/network_security_config.xml`

- Strict certificate validation using system CAs only
- No user certificate trust for maximum security
- Requires proper CA-signed certificates
- Production domain placeholder needs to be updated

## Android App Implementation

### HTTP Client Creation

The `MainActivity.createHttpClient()` method automatically selects the appropriate client:

```kotlin
private fun createHttpClient(): OkHttpClient {
    return if (BuildConfig.DEBUG) {
        createDebugHttpClient()  // Allows self-signed certs
    } else {
        createReleaseHttpClient() // Strict validation
    }
}
```

### Debug Client Features

- Logs certificate acceptance for debugging
- Accepts self-signed certificates
- Flexible hostname verification
- Development-friendly error handling

### Release Client Features

- Uses system certificate authorities only
- Respects network security configuration
- Strict hostname verification
- Production-grade security

## Production Deployment Requirements

### 1. Backend Certificate Setup

For production deployment, replace the self-signed certificate in `app-backend/` with proper CA-signed certificates:

```bash
# Replace these files with CA-signed certificates:
app-backend/cert.pem  # Public certificate
app-backend/key.pem   # Private key
```

### 2. Certificate Options

**Option A: Let's Encrypt (Free)**
```bash
certbot certonly --standalone -d your-domain.com
cp /etc/letsencrypt/live/your-domain.com/fullchain.pem app-backend/cert.pem
cp /etc/letsencrypt/live/your-domain.com/privkey.pem app-backend/key.pem
```

**Option B: Commercial CA**
- Purchase certificate from trusted CA (DigiCert, Comodo, etc.)
- Follow CA's installation instructions
- Place certificate files in `app-backend/`

**Option C: Internal CA (Enterprise)**
- Use organizational certificate authority
- Ensure certificates are trusted by target devices
- Consider certificate pinning for additional security

### 3. Update Production Domain

Edit `demo-app/app/src/release/res/xml/network_security_config.xml`:

```xml
<domain includeSubdomains="true">your-actual-domain.com</domain>
```

### 4. Android App Configuration

Update the default backend URL in `SettingsActivity` to use the production domain:

```kotlin
const val DEFAULT_BACKEND_URL = "https://your-production-domain.com:8443"
```

## Security Verification

### Testing Debug Build

1. Build debug APK: `./gradlew assembleDebug`
2. Install on test device
3. Verify self-signed certificate acceptance
4. Check logs for certificate debugging messages

### Testing Release Build

1. Build release APK: `./gradlew assembleRelease`
2. Install on test device
3. Verify rejection of self-signed certificates
4. Confirm only CA-signed certificates are accepted

### Certificate Validation Test

```bash
# Test certificate chain
openssl s_client -connect your-domain.com:8443 -showcerts

# Verify certificate expiration
openssl x509 -in app-backend/cert.pem -text -noout | grep "Not After"
```

## Security Best Practices

1. **Never deploy debug builds to production**
2. **Use HTTPS everywhere** - no HTTP in production
3. **Monitor certificate expiration** - set up renewal alerts
4. **Consider certificate pinning** for high-security applications
5. **Regular security audits** of certificate configuration
6. **Backup certificate private keys** securely

## Troubleshooting

### Common Issues

**Certificate not trusted in release build**
- Verify certificate is signed by trusted CA
- Check network_security_config.xml domain configuration
- Ensure certificate matches domain name

**Debug build not accepting self-signed certificate**
- Verify debug network_security_config.xml exists
- Check build type configuration
- Review OkHttp client implementation

**Production deployment fails**
- Verify certificate and private key match
- Check certificate expiration date
- Ensure proper file permissions on server

### Logging

The app provides detailed logging for certificate validation:

- Debug builds: Certificate acceptance logged
- Release builds: Strict validation logged
- Network errors: Detailed error messages

## Monitoring

For production deployments, monitor:

- Certificate expiration dates
- SSL/TLS handshake failures
- Certificate validation errors
- Client connection success rates

---

**Last Updated**: January 2025  
**Security Level**: Production Ready  
**Next Review**: Certificate expiration check
