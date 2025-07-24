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
            val token = task.result
            Log.d(TAG, "Firebase Registration Token: $token")
            
            // Send token to server
            sendTokenToServer(token)
        }
    }
    
    private fun sendTokenToServer(token: String) {
        updateStatus(getString(R.string.status_encrypting_token))
        
        // Encrypt the token before sending
        val encryptedToken = try {
            encryptToken(token)
        } catch (e: Exception) {
            Log.e(TAG, "Failed to encrypt token", e)
            updateStatus(getString(R.string.status_failed_encrypt, e.message ?: ""))
            registerButton.isEnabled = true
            return
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
        // Generate a random AES-256 key for this token
        val keyGen = KeyGenerator.getInstance("AES")
        keyGen.init(256)
        val aesKey = keyGen.generateKey()
        
        // Encrypt the token with AES-GCM
        val aesCipher = Cipher.getInstance("AES/GCM/NoPadding")
        val iv = ByteArray(12) // 96-bit IV for GCM
        SecureRandom().nextBytes(iv)
        val gcmSpec = GCMParameterSpec(128, iv) // 128-bit authentication tag
        aesCipher.init(Cipher.ENCRYPT_MODE, aesKey, gcmSpec)
        
        val encryptedToken = aesCipher.doFinal(token.toByteArray())
        
        // Encrypt the AES key with RSA
        val publicKey = loadPublicKey()
        val rsaCipher = Cipher.getInstance("RSA/ECB/PKCS1Padding")
        rsaCipher.init(Cipher.ENCRYPT_MODE, publicKey)
        val encryptedAesKey = rsaCipher.doFinal(aesKey.encoded)
        
        // Combine: IV (12 bytes) + encrypted AES key length (4 bytes) + encrypted AES key + encrypted token
        val keyLengthBytes = ByteArray(4)
        keyLengthBytes[0] = (encryptedAesKey.size shr 24).toByte()
        keyLengthBytes[1] = (encryptedAesKey.size shr 16).toByte()
        keyLengthBytes[2] = (encryptedAesKey.size shr 8).toByte()
        keyLengthBytes[3] = encryptedAesKey.size.toByte()
        
        val combined = iv + keyLengthBytes + encryptedAesKey + encryptedToken
        return Base64.encodeToString(combined, Base64.DEFAULT)
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
