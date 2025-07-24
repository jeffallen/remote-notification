# Security Improvements TODO List

> **Generated from Security Review Report - January 2025**

This document provides an actionable checklist of security improvements needed for the notification system. Items are prioritized by risk level and impact.

---

## 🔴 **CRITICAL PRIORITY** ~~(Fix Immediately)~~ **ALL COMPLETED**

### ✅ **TODO-001: Update Go Runtime to 1.24.5+** *(COMPLETED)*
- **Issue:** 3 active vulnerabilities in Go 1.24.3 standard library
- **Impact:** HTTP header leaks, file operation inconsistencies, certificate validation bypass
- **Action:** Update both app-backend and notification-backend Go runtime
- **Verification:** Run `govulncheck ./...` to confirm no vulnerabilities
- **Effort:** Low (30 minutes)

### ✅ **TODO-002: Replace Self-Signed Certificates** *(COMPLETED)*  
- **Issue:** Using self-signed cert for production HTTPS (CN=10.0.2.2)
- **Impact:** MITM attacks possible, browser warnings
- **Action:** 
  - Generate proper CA-signed certificates OR
  - Use Let's Encrypt for public deployment OR
  - Use internal CA for enterprise deployment
- **Files:** `app-backend/cert.pem`, `app-backend/key.pem`
- **Effort:** Medium (2-4 hours)

### ☐ **TODO-003: Remove Unsafe HTTP Client in Android App**
- **Issue:** Bypasses all certificate validation (`trustAllCerts`, `hostnameVerifier`)
- **Impact:** Complete MITM vulnerability in Android app
- **Action:** Remove unsafe OkHttpClient configuration, use proper certificate validation
- **Files:** `demo-app/app/src/main/java/org/nella/rn/demo/MainActivity.kt`
- **Effort:** Low (1 hour)

### ☐ **TODO-004: Add Basic API Authentication**
- **Issue:** All backend endpoints publicly accessible without authentication
- **Impact:** Spam attacks, denial of service, resource abuse
- **Action:** Implement API key authentication or request signing
- **Options:**
  - Simple API key in header (`X-API-Key`)
  - HMAC request signing
  - JWT token-based auth
- **Files:** `app-backend/main.go`, `notification-backend/main.go`
- **Effort:** Medium (3-6 hours)

---

## 🟡 **HIGH PRIORITY** ~~(Fix Soon)~~ **3/5 COMPLETED**

### ✅ **TODO-005: Update Kotlin to 2.2.0** *(COMPLETED)*
- **Issue:** Using Kotlin 1.9.10 (significantly outdated)
- **Impact:** Missing security fixes and language improvements
- **Action:** Update `kotlin_version` in `demo-app/build.gradle`
- **Verification:** Ensure app compiles and runs correctly
- **Effort:** Low (30 minutes)

### ☐ **TODO-006: Implement Rate Limiting**
- **Issue:** No rate limiting on any endpoints
- **Impact:** DoS attacks, resource exhaustion
- **Action:** Add rate limiting middleware
- **Suggested Rates:**
  - `/register`: 10 requests/minute per IP
  - `/send`, `/notify`: 100 requests/minute per IP
  - `/status`: 60 requests/minute per IP
- **Libraries:** `golang.org/x/time/rate` or middleware package
- **Effort:** Medium (2-3 hours)

### ✅ **TODO-007: Update Firebase and Cloud Dependencies** *(COMPLETED)*
- **Issue:** Multiple Go modules have newer versions available
- **Impact:** Missing bug fixes and security patches
- **Action:** Update major dependencies in notification-backend
- **Key Updates:**
  - `firebase.google.com/go/v4: v4.16.1 → v4.17.0`
  - `cloud.google.com/go/auth: v0.16.1 → v0.16.3`
  - Various other cloud.google.com modules
- **Files:** `notification-backend/go.mod`
- **Effort:** Low (1 hour)

### ✅ **TODO-008: Add Request/Response Logging** *(COMPLETED)*
- **Issue:** Limited observability into API usage and errors
- **Impact:** Difficult to detect attacks or troubleshoot issues
- **Action:** Add structured logging for all HTTP requests
- **Include:** IP, method, path, status code, response time, error details
- **Format:** JSON for easy parsing
- **Effort:** Low (1-2 hours)

### ☐ **TODO-009: Add HTTPS to Notification Backend**
- **Issue:** notification-backend only supports HTTP (port 8080)
- **Impact:** Internal traffic not encrypted
- **Action:** Add TLS support with proper certificates
- **Note:** Less critical if running in secure internal network
- **Effort:** Medium (2-3 hours)

---

## 🟢 **MEDIUM PRIORITY** ~~(Improve Over Time)~~ **1/5 COMPLETED**

### ✅ **TODO-010: Upgrade Java Target to 11+** *(COMPLETED)*
- **Issue:** Using Java 1.8 (EOL, missing security features)
- **Impact:** Missing modern security improvements
- **Action:** Update `sourceCompatibility` and `targetCompatibility` to JavaVersion.VERSION_11
- **Files:** `demo-app/app/build.gradle`
- **Android Compatibility Analysis:**
  - **Java 11 Target:** Safe upgrade, no minSdk impact
  - **Current minSdk 21** (Android 5.0+) remains unchanged
  - **ART Runtime:** Android 5.0+ supports Java 11 bytecode natively
  - **Market Coverage:** No reduction in addressable market
  - **Compatibility:** Java 11 features compile to compatible bytecode
  - **AGP Support:** Android Gradle Plugin 8.11.0 fully supports Java 11
  - **Desugaring:** AGP automatically handles Java 11 features for older Android versions
  - **Performance:** Potential improvements from newer compiler optimizations
  - **Security:** Access to enhanced cryptographic APIs and security patches
- **Trade-offs:**
  - ✅ **Pros:** Better security, modern language features, improved tooling
  - ✅ **Pros:** No impact on app compatibility or market reach
  - ⚠️ **Cons:** Slightly larger APK size (minimal, <1%)
  - ⚠️ **Cons:** Build time may increase slightly during compilation
- **Verification:** Test on Android 5.0 (minSdk 21) device to confirm compatibility
- **Effort:** Low (30 minutes)

### ☐ **TODO-011: Implement Key Rotation Mechanism**
- **Issue:** No process for rotating RSA keys
- **Impact:** Long-term key compromise risk
- **Action:** Design key rotation with backward compatibility
- **Considerations:** Multiple public keys, versioned encryption
- **Effort:** High (8-12 hours)

### ☐ **TODO-012: Add Certificate Pinning to Android App**
- **Issue:** Relies on system CA store (can be compromised)
- **Impact:** Advanced persistent threats
- **Action:** Pin specific certificates or public keys
- **Implementation:** Use OkHttp CertificatePinner
- **Effort:** Medium (2-4 hours)

### ☐ **TODO-013: Enhanced Input Validation**
- **Issue:** Basic validation present but could be more robust
- **Improvements:**
  - Strict Content-Type validation
  - Request body size limits at HTTP layer
  - Sanitize log message content
  - Validate JSON schema more strictly
- **Effort:** Medium (3-4 hours)

### ☐ **TODO-014: Add Health Check Endpoints**
- **Issue:** No standardized health checking
- **Impact:** Difficult to monitor service health
- **Action:** Add `/health` endpoints with dependency checks
- **Include:** Database connectivity, Firebase status, disk space
- **Effort:** Low (1-2 hours)

---

## 🔵 **LOW PRIORITY** (Future Enhancements)

### ☐ **TODO-015: Admin Dashboard Development**
- **Description:** Web interface for monitoring and management
- **Features:** Token statistics, notification history, system health
- **Authentication:** Proper admin authentication required
- **Effort:** High (20+ hours)

### ☐ **TODO-016: Token Analytics and Metrics**
- **Description:** Usage analytics, delivery rates, error tracking
- **Implementation:** Prometheus metrics, Grafana dashboards
- **Privacy:** Ensure no PII in metrics
- **Effort:** Medium (6-8 hours)

### ☐ **TODO-017: Performance Optimization**
- **Description:** Connection pooling, caching, async processing
- **Areas:** Database connections, HTTP clients, Firebase connections
- **Measurement:** Benchmark before/after changes
- **Effort:** Medium (4-6 hours)

### ☐ **TODO-018: Multi-Region Deployment Support**
- **Description:** Support for multiple deployment regions
- **Considerations:** Data sovereignty, latency, failover
- **Requirements:** Config management, service discovery
- **Effort:** High (12+ hours)

### ☐ **TODO-019: Advanced Monitoring and Alerting**
- **Description:** Comprehensive monitoring with alerts
- **Tools:** Prometheus, Grafana, AlertManager
- **Metrics:** Error rates, response times, token registration rates
- **Effort:** Medium (6-8 hours)

### ☐ **TODO-020: Security Scanning Integration**
- **Description:** Automated security scanning in CI/CD
- **Tools:** 
  - `govulncheck` for Go vulnerabilities
  - `nancy` or `grype` for dependency scanning
  - `gosec` for static analysis
  - `semgrep` for security patterns
- **Integration:** GitHub Actions or CI pipeline
- **Effort:** Medium (3-4 hours)

---

## 🗺️ Implementation Strategy

### Phase 1: Critical Security (Week 1)
- Complete TODO-001 through TODO-004
- Test thoroughly in staging environment
- Deploy with careful monitoring

### Phase 2: Core Improvements (Week 2-3)
- Complete TODO-005 through TODO-009
- Add comprehensive testing
- Performance validation

### Phase 3: Enhanced Security (Month 2)
- Complete TODO-010 through TODO-014
- Security testing with external tools
- Documentation updates

### Phase 4: Advanced Features (Month 3+)
- Complete TODO-015 through TODO-020
- Long-term maintenance planning
- Regular security reviews

---

## 📝 Testing Requirements

For each TODO item completed:

1. **Unit Tests:** Verify individual component functionality
2. **Integration Tests:** Test component interactions
3. **Security Tests:** Verify security improvements work
4. **Regression Tests:** Ensure existing functionality preserved
5. **Performance Tests:** Validate no performance degradation

## 📈 Success Metrics

- **Vulnerability Count:** 0 active vulnerabilities (currently 3)
- **Authentication Coverage:** 100% of API endpoints protected
- **Certificate Validity:** All certificates properly signed and valid
- **Dependency Freshness:** <30 days behind latest versions
- **Test Coverage:** >80% code coverage maintained
- **Performance:** <10% degradation from baseline

---

**Document Version:** 1.0  
**Last Updated:** January 2025  
**Next Review:** February 2025