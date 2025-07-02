package org.nella.fcmapp

import android.os.Bundle
import android.util.Log
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
import java.security.spec.X509EncodedKeySpec
import java.util.Base64
import javax.crypto.Cipher
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
        private const val REGISTER_URL = "https://10.0.2.2:8443/register"
    }
    
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_main)
        
        registerButton = findViewById(R.id.registerButton)
        statusText = findViewById(R.id.statusText)
        
        registerButton.setOnClickListener {
            registerDeviceToken()
        }
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
        json.put("token", encryptedToken)
        json.put("platform", "android")
        json.put("encrypted", true)
        
        val body = json.toString().toRequestBody("application/json; charset=utf-8".toMediaType())
        
        val request = Request.Builder()
            .url(REGISTER_URL)
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
                        updateStatus("Token registered successfully!\nToken: ${token.take(20)}...")
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
        val publicKey = loadPublicKey()
        val cipher = Cipher.getInstance("RSA/ECB/PKCS1Padding")
        cipher.init(Cipher.ENCRYPT_MODE, publicKey)
        
        val encryptedBytes = cipher.doFinal(token.toByteArray())
        return Base64.getEncoder().encodeToString(encryptedBytes)
    }
    
    private fun loadPublicKey(): PublicKey {
        val publicKeyPem = assets.open("public_key.pem").bufferedReader().use { it.readText() }
        
        // Remove PEM headers and footers, and newlines
        val publicKeyBase64 = publicKeyPem
            .replace("-----BEGIN PUBLIC KEY-----", "")
            .replace("-----END PUBLIC KEY-----", "")
            .replace("\n", "")
            .replace("\r", "")
        
        val keyBytes = Base64.getDecoder().decode(publicKeyBase64)
        val keySpec = X509EncodedKeySpec(keyBytes)
        val keyFactory = KeyFactory.getInstance("RSA")
        
        return keyFactory.generatePublic(keySpec)
    }
}
