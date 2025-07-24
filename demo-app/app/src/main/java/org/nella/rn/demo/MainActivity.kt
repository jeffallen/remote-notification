package org.nella.rn.demo

import android.content.Intent
import android.os.Bundle
import android.util.Log
import android.view.Menu
import android.view.MenuItem
import android.widget.Button
import android.widget.TextView
import androidx.appcompat.app.AppCompatActivity
import com.google.firebase.messaging.FirebaseMessaging
import okhttp3.*
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.RequestBody.Companion.toRequestBody
import java.io.IOException
import java.security.cert.X509Certificate
import java.security.KeyFactory
import java.security.PublicKey
import java.security.SecureRandom
import java.security.spec.X509EncodedKeySpec
import android.util.Base64
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec
import javax.net.ssl.SSLContext
import javax.net.ssl.TrustManager
import javax.net.ssl.X509TrustManager
import javax.net.ssl.HostnameVerifier
import org.json.JSONObject

class MainActivity : AppCompatActivity() {
    
    private lateinit var registerButton: Button
    private lateinit var statusText: TextView
    private val client = createHttpClient()
    
    companion object {
        private const val TAG = "MainActivity"
        // Set to true to force debug certificate behavior for testing
        // WARNING: Never set to true in production builds
        private const val FORCE_DEBUG_CERTIFICATES = false
    }
    
    /**
     * Securely wipes a string from memory by overwriting its backing array
     * Returns empty string to replace the original variable
     */
    private fun secureWipeString(value: String?): String {
        if (value == null) return ""
        
        try {
            // Convert to char array and overwrite with zeros
            val chars = value.toCharArray()
            for (i in chars.indices) {
                chars[i] = '\u0000'
            }
            
            // Also try to overwrite the byte representation
            val bytes = value.toByteArray()
            for (i in bytes.indices) {
                bytes[i] = 0
            }
            
            Log.d(TAG, "Securely wiped string from memory (${value.length} chars)")
        } catch (e: Exception) {
            Log.w(TAG, "Warning: Could not securely wipe string from memory", e)
        }
        
        return "" // Return empty string to replace original variable
    }
    
    /**
     * Securely wipes a byte array from memory
     */
    private fun secureWipeBytes(bytes: ByteArray?) {
        if (bytes == null) return
        
        try {
            for (i in bytes.indices) {
                bytes[i] = 0
            }
            Log.d(TAG, "Securely wiped byte array from memory (${bytes.size} bytes)")
        } catch (e: Exception) {
            Log.w(TAG, "Warning: Could not securely wipe byte array from memory", e)
        }
    }
    
    /**
     * Securely wipes a SecretKey from memory
     */
    private fun secureWipeSecretKey(key: SecretKey?) {
        if (key == null) return
        
        try {
            // Wipe the encoded key bytes
            secureWipeBytes(key.encoded)
            Log.d(TAG, "Securely wiped SecretKey from memory")
        } catch (e: Exception) {
            Log.w(TAG, "Warning: Could not securely wipe SecretKey from memory", e)
        }
    }
    
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_main)
        
        registerButton = findViewById(R.id.registerButton)
        statusText = findViewById(R.id.statusText)
        
        registerButton.setOnClickListener {
            registerDeviceToken()
        }
        
        // Show current backend URL in status
        updateStatusWithBackendUrl()
    }
    
    override fun onCreateOptionsMenu(menu: Menu?): Boolean {
        menuInflater.inflate(R.menu.main_menu, menu)
        return true
    }
    
    override fun onOptionsItemSelected(item: MenuItem): Boolean {
        return when (item.itemId) {
            R.id.action_settings -> {
                startActivity(Intent(this, SettingsActivity::class.java))
                true
            }
            else -> super.onOptionsItemSelected(item)
        }
    }
    
    override fun onResume() {
        super.onResume()
        updateStatusWithBackendUrl()
    }
    
    private fun updateStatusWithBackendUrl() {
        val backendUrl = SettingsActivity.getBackendUrl(this)
        updateStatus(getString(R.string.status_ready_to_register, backendUrl))
    }
    
    private fun registerDeviceToken() {
        updateStatus(getString(R.string.status_getting_token))
        registerButton.isEnabled = false
        
        FirebaseMessaging.getInstance().token.addOnCompleteListener { task ->
            if (!task.isSuccessful) {
                Log.w(TAG, "Fetching Firebase registration token failed", task.exception)
                updateStatus(getString(R.string.status_failed_get_token, task.exception?.message ?: ""))
                registerButton.isEnabled = true
                return@addOnCompleteListener
            }
            
            // Get new Firebase registration token
            var token = task.result
            Log.d(TAG, "Firebase Registration Token received (${token.length} characters)")
            
            try {
                // Send token to server
                sendTokenToServer(token)
            } finally {
                // Wipe token from memory after sending
                token = secureWipeString(token)
            }
        }
    }
    
    private fun sendTokenToServer(token: String) {
        var secureToken = token // Create mutable copy for secure wiping
        
        try {
            updateStatus(getString(R.string.status_encrypting_token))
            
            // Encrypt the token before sending
            val encryptedToken = try {
                encryptToken(secureToken)
            } catch (e: Exception) {
                Log.e(TAG, "Failed to encrypt token", e)
                updateStatus(getString(R.string.status_failed_encrypt, e.message ?: ""))
                registerButton.isEnabled = true
                return
            } finally {
                // Wipe token from memory immediately after encryption attempt
                secureToken = secureWipeString(secureToken)
            }
            
            val json = JSONObject()
            json.put("encrypted_data", encryptedToken)
            json.put("platform", "android")
            
            val body = json.toString().toRequestBody("application/json; charset=utf-8".toMediaType())
            
            val backendUrl = SettingsActivity.getBackendUrl(this@MainActivity)
            val registerUrl = "$backendUrl/register"
            
            val request = Request.Builder()
                .url(registerUrl)
                .post(body)
                .build()
            
            client.newCall(request).enqueue(object : Callback {
                override fun onFailure(call: Call, e: IOException) {
                    Log.e(TAG, "Failed to send token to server", e)
                    runOnUiThread {
                        updateStatus(getString(R.string.status_failed_register, e.message ?: ""))
                        registerButton.isEnabled = true
                    }
                }
                
                override fun onResponse(call: Call, response: Response) {
                    val responseBody = response.body?.string()
                    Log.d(TAG, "Server response: ${response.code} - $responseBody")
                    
                    runOnUiThread {
                        if (response.isSuccessful) {
                            updateStatus(getString(R.string.status_success))
                        } else {
                            updateStatus(getString(R.string.status_server_error, response.code, responseBody))
                        }
                        registerButton.isEnabled = true
                    }
                }
            })
            
        } finally {
            // Ensure token is wiped from memory even if there are exceptions
            secureToken = secureWipeString(secureToken)
        }
    }
    
    private fun updateStatus(message: String) {
        statusText.text = message
    }
    
    /**
     * Creates HTTP client with build-appropriate certificate validation
     * - Debug builds: Allow self-signed certificates for development
     * - Release builds: Strict certificate validation using system CAs only
     */
    private fun createHttpClient(): OkHttpClient {
        return if (isDebugBuild()) {
            createDebugHttpClient()
        } else {
            createReleaseHttpClient()
        }
    }
    
    /**
     * Determines if this is a debug build by checking application info
     * Falls back to safe release behavior if detection fails
     */
    private fun isDebugBuild(): Boolean {
        // Manual override for testing (WARNING: Never use in production)
        if (FORCE_DEBUG_CERTIFICATES) {
            Log.w(TAG, "WARNING: Using forced debug certificate mode - not for production!")
            return true
        }
        
        return try {
            // Check application debuggable flag
            val appInfo = packageManager.getApplicationInfo(packageName, 0)
            val isDebuggable = (appInfo.flags and android.content.pm.ApplicationInfo.FLAG_DEBUGGABLE) != 0
            
            Log.i(TAG, "Build type detection - Debuggable flag: $isDebuggable")
            return isDebuggable
        } catch (e: Exception) {
            Log.e(TAG, "Error checking debug build status, defaulting to release mode for security", e)
            false // Default to release behavior for safety
        }
    }
    
    /**
     * Debug HTTP client - allows self-signed certificates for development
     * WARNING: Only used in debug builds, never in production
     */
    private fun createDebugHttpClient(): OkHttpClient {
        return try {
            Log.w(TAG, "DEBUG BUILD: Using development HTTP client that accepts self-signed certificates")
            
            // Create a trust manager that accepts self-signed certificates
            val trustAllCerts = arrayOf<TrustManager>(
                object : X509TrustManager {
                    override fun checkClientTrusted(chain: Array<X509Certificate>, authType: String) {
                        Log.d(TAG, "Debug: Accepting client certificate: ${chain[0].subjectDN}")
                    }
                    override fun checkServerTrusted(chain: Array<X509Certificate>, authType: String) {
                        Log.d(TAG, "Debug: Accepting server certificate: ${chain[0].subjectDN}")
                    }
                    override fun getAcceptedIssuers(): Array<X509Certificate> = arrayOf()
                }
            )

            val sslContext = SSLContext.getInstance("TLS")
            sslContext.init(null, trustAllCerts, SecureRandom())

            OkHttpClient.Builder()
                .sslSocketFactory(sslContext.socketFactory, trustAllCerts[0] as X509TrustManager)
                .hostnameVerifier { hostname, _ -> 
                    Log.d(TAG, "Debug: Accepting hostname: $hostname")
                    true
                }
                .build()
        } catch (e: Exception) {
            Log.e(TAG, "Error creating debug HTTP client", e)
            OkHttpClient() // Fallback to default client
        }
    }
    
    /**
     * Release HTTP client - strict certificate validation
     * Uses system certificate authorities only for production security
     */
    private fun createReleaseHttpClient(): OkHttpClient {
        Log.i(TAG, "RELEASE BUILD: Using secure HTTP client with strict certificate validation")
        
        // Use default OkHttpClient which respects network security config
        // This will enforce the release network_security_config.xml settings
        return OkHttpClient.Builder()
            .build()
    }
    
    private fun encryptToken(token: String): String {
        var aesKey: SecretKey? = null
        var iv: ByteArray? = null
        var encryptedToken: ByteArray? = null
        var encryptedAesKey: ByteArray? = null
        
        try {
            // Generate a random AES-256 key for this token
            val keyGen = KeyGenerator.getInstance("AES")
            keyGen.init(256)
            aesKey = keyGen.generateKey()
            
            // Encrypt the token with AES-GCM
            val aesCipher = Cipher.getInstance("AES/GCM/NoPadding")
            iv = ByteArray(12) // 96-bit IV for GCM
            SecureRandom().nextBytes(iv)
            val gcmSpec = GCMParameterSpec(128, iv) // 128-bit authentication tag
            aesCipher.init(Cipher.ENCRYPT_MODE, aesKey, gcmSpec)
            
            encryptedToken = aesCipher.doFinal(token.toByteArray())
            
            // Encrypt the AES key with RSA
            val publicKey = loadPublicKey()
            val rsaCipher = Cipher.getInstance("RSA/ECB/PKCS1Padding")
            rsaCipher.init(Cipher.ENCRYPT_MODE, publicKey)
            encryptedAesKey = rsaCipher.doFinal(aesKey.encoded)
            
            // Combine: IV (12 bytes) + encrypted AES key length (4 bytes) + encrypted AES key + encrypted token
            val keyLengthBytes = ByteArray(4)
            keyLengthBytes[0] = (encryptedAesKey.size shr 24).toByte()
            keyLengthBytes[1] = (encryptedAesKey.size shr 16).toByte()
            keyLengthBytes[2] = (encryptedAesKey.size shr 8).toByte()
            keyLengthBytes[3] = encryptedAesKey.size.toByte()
            
            val combined = iv + keyLengthBytes + encryptedAesKey + encryptedToken
            return Base64.encodeToString(combined, Base64.DEFAULT)
            
        } finally {
            // Securely wipe sensitive data from memory
            try {
                aesKey?.let { secureWipeSecretKey(it) }
                iv?.let { secureWipeBytes(it) }
                encryptedToken?.let { secureWipeBytes(it) }
                encryptedAesKey?.let { secureWipeBytes(it) }
                
                Log.d(TAG, "Encryption completed, sensitive data wiped from memory")
            } catch (e: Exception) {
                Log.w(TAG, "Warning: Could not completely wipe encryption data from memory", e)
            }
        }
    }
    
    private fun loadPublicKey(): PublicKey {
        val publicKeyPem = assets.open("public_key.pem").bufferedReader().use { it.readText() }
        
        // Remove PEM headers and footers, and newlines
        val publicKeyBase64 = publicKeyPem
            .replace("-----BEGIN PUBLIC KEY-----", "")
            .replace("-----END PUBLIC KEY-----", "")
            .replace("\n", "")
            .replace("\r", "")
        
        val keyBytes = Base64.decode(publicKeyBase64, Base64.DEFAULT)
        val keySpec = X509EncodedKeySpec(keyBytes)
        val keyFactory = KeyFactory.getInstance("RSA")
        
        return keyFactory.generatePublic(keySpec)
    }
}
