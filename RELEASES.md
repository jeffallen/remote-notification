# Release Process

This document explains how to create and deploy releases for the minnotif-android project.

## Creating a Release

### 1. Create a Git Tag

To trigger a release build, create and push a git tag:

```bash
# Create a new tag (use semantic versioning)
git tag v1.0.0

# Push the tag to GitHub
git push origin v1.0.0
```

### 2. Automatic Build Process

The GitHub Actions workflow will automatically:

1. **Build Android APK** - Creates a release APK for sideloading
2. **Build Go Binaries** - Cross-compiles servers for Linux, macOS, and Windows
3. **Create GitHub Release** - Publishes files as downloadable release assets

### 3. Release Artifacts

Each release includes:

- **Android APK**: `minnotif-android-v1.0.0-unsigned.apk` (or signed if keystore configured)
- **Go Binaries**: Cross-compiled for multiple platforms:
  - `app-backend-linux-amd64`
  - `app-backend-darwin-amd64` 
  - `app-backend-windows-amd64.exe`
  - `notification-backend-linux-amd64`
  - `notification-backend-darwin-amd64`
  - `notification-backend-windows-amd64.exe`
  - ARM64 variants for Linux and macOS

## Installing the APK

### On Your Android Device:

1. **Download** the APK from the GitHub release page
2. **Enable Unknown Sources**:
   - Go to Settings > Security
   - Enable "Install from Unknown Sources" or "Install unknown apps"
3. **Install APK**:
   - Open the downloaded APK file
   - Tap "Install" when prompted
   - Accept permissions

### Security Notes:

- APKs are unsigned by default (safe for personal use)
- Enable signing by adding keystore secrets (see below)
- The app uses end-to-end encryption for all token transmission

## APK Signing (Optional)

To create signed APKs for distribution:

### 1. Create a Keystore

```bash
# Generate a release keystore
keytool -genkey -v -keystore release-keystore.jks \
  -keyalg RSA -keysize 2048 -validity 10000 \
  -alias release-key
```

### 2. Add GitHub Secrets

Add these secrets to your GitHub repository:

- `KEYSTORE_BASE64`: Base64-encoded keystore file
  ```bash
  base64 -i release-keystore.jks | pbcopy
  ```
- `KEYSTORE_PASSWORD`: Keystore password
- `KEY_ALIAS`: Key alias (e.g., "release-key")
- `KEY_PASSWORD`: Key password

### 3. Signed APK Output

With secrets configured, releases will include:
- `minnotif-android-v1.0.0-signed.apk` - Signed APK ready for distribution

## Deploying Go Servers

### Download and Install:

```bash
# Download the appropriate binary for your platform
wget https://github.com/your-org/minnotif-android/releases/download/v1.0.0/app-backend-linux-amd64
wget https://github.com/your-org/minnotif-android/releases/download/v1.0.0/notification-backend-linux-amd64

# Make executable
chmod +x app-backend-linux-amd64 notification-backend-linux-amd64

# Install to system
sudo mv app-backend-linux-amd64 /usr/bin/app-backend
sudo mv notification-backend-linux-amd64 /usr/bin/notification-backend

# Use systemd services from the repository
sudo cp systemd/*.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable app-backend notification-backend
sudo systemctl start app-backend notification-backend
```

## Version Management

### Semantic Versioning

Use [semantic versioning](https://semver.org/) for tags:

- `v1.0.0` - Major release
- `v1.1.0` - Minor feature addition
- `v1.0.1` - Patch/bugfix

### Pre-releases

For testing releases:

```bash
git tag v1.1.0-beta.1
git push origin v1.1.0-beta.1
```

### Release Notes

The GitHub Action automatically generates release notes with:
- Download instructions
- Security information
- Deployment checklist
- File descriptions

## Troubleshooting

### APK Installation Issues

- **"App not installed"**: Enable unknown sources, check storage space
- **"Parse error"**: APK may be corrupted, re-download
- **Permission denied**: Check Android version compatibility

### Build Failures

- **Gradle issues**: Check Android SDK setup in workflow
- **Signing errors**: Verify keystore secrets are correct
- **Go build errors**: Check Go version and dependencies

### Server Deployment Issues

- **Permission denied**: Use `sudo` for system installation
- **Service fails**: Check systemd logs with `journalctl -u service-name`
- **Network issues**: Verify firewall and port configuration

## Development Workflow

1. **Develop** features on feature branches
2. **Test** with development builds
3. **Merge** to main branch
4. **Tag** stable versions for release
5. **Deploy** servers and distribute APK
6. **Monitor** logs and user feedback

This automated release process ensures consistent, reproducible builds for both Android and server components.
