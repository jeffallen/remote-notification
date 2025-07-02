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
import org.json.JSONObject

class MainActivity : AppCompatActivity() {
    
    private lateinit var registerButton: Button
    private lateinit var statusText: TextView
    private val client = OkHttpClient()
    
    companion object {
        private const val TAG = "MainActivity"
        private const val REGISTER_URL = "http://10.0.2.2:8081/register"
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
        updateStatus("Sending token to server...")
        
        val json = JSONObject()
        json.put("token", token)
        json.put("platform", "android")
        
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
}
