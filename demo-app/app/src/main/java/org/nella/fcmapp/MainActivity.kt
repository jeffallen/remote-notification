package org.nella.fcmapp

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
    private val client = createUnsafeOkHttpClient()
    
    companion object {
        private const val TAG = "MainActivity"
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
        updateStatus("Ready to register with backend: $backendUrl")
    }
    
    private fun registerDeviceToken() {
        updateStatus("Getting FCM token...")
        registerButton.isEnabled = false
        
        FirebaseMessaging.getInstance().token.addOnCompleteListener { task ->
            if (!task.isSuccessful) {
                Log.w(TAG, "Fetching FCM registration token failed", task.exception)
                updateStatus("Failed to get FCM token: ${task.exception?.message}")
                registerButton.isEnabled = true
                return@addOnCompleteListener
            }
            
            // Get new FCM registration token
            val token = task.result
            Log.d(TAG, "FCM Registration Token: $token")
            
            // Send token to server
            sendTokenToServer(token)
        }
    }
    
    private fun sendTokenToServer(token: String) {
        updateStatus("Encrypting and sending token to server...")
        
        // Encrypt the token before sending
        val encryptedToken = try {
            encryptToken(token)
        } catch (e: Exception) {
            Log.e(TAG, "Failed to encrypt token", e)
            updateStatus("Failed to encrypt token: ${e.message}")
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
                    updateStatus("Failed to register token: ${e.message}")
                    registerButton.isEnabled = true
                }
            }
            
            override fun onResponse(call: Call, response: Response) {
                val responseBody = response.body?.string()
                Log.d(TAG, "Server response: ${response.code} - $responseBody")
                
                runOnUiThread {
                    if (response.isSuccessful) {
                        updateStatus("Encrypted token registered successfully!")
                    } else {
                        updateStatus("Server error: ${response.code}\n$responseBody")
                    }
                    registerButton.isEnabled = true
                }
            }
        })
    }
    
    private fun updateStatus(message: String) {
        statusText.text = message
    }
    
    private fun createUnsafeOkHttpClient(): OkHttpClient {
        return try {
            // Create a trust manager that does not validate certificate chains
            val trustAllCerts = arrayOf<TrustManager>(
                object : X509TrustManager {
                    override fun checkClientTrusted(chain: Array<X509Certificate>, authType: String) {}
                    override fun checkServerTrusted(chain: Array<X509Certificate>, authType: String) {}
                    override fun getAcceptedIssuers(): Array<X509Certificate> = arrayOf()
                }
            )

            // Install the all-trusting trust manager
            val sslContext = SSLContext.getInstance("SSL")
            sslContext.init(null, trustAllCerts, java.security.SecureRandom())

            // Create an ssl socket factory with our all-trusting manager
            val sslSocketFactory = sslContext.socketFactory

            OkHttpClient.Builder()
                .sslSocketFactory(sslSocketFactory, trustAllCerts[0] as X509TrustManager)
                .hostnameVerifier(HostnameVerifier { _, _ -> true })
                .build()
        } catch (e: Exception) {
            Log.e(TAG, "Error creating unsafe HTTP client", e)
            OkHttpClient()
        }
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
