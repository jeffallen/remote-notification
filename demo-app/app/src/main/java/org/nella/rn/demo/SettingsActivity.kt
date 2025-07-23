package org.nella.rn.demo

import android.content.Context
import android.content.SharedPreferences
import android.os.Bundle
import android.util.Patterns
import android.widget.Button
import android.widget.EditText
import android.widget.Toast
import androidx.appcompat.app.AppCompatActivity

class SettingsActivity : AppCompatActivity() {
    
    private lateinit var urlEditText: EditText
    private lateinit var saveButton: Button
    private lateinit var resetButton: Button
    private lateinit var prefs: SharedPreferences
    
    companion object {
        const val PREFS_NAME = "RnDemoSettings"
        const val PREF_BACKEND_URL = "backend_url"
        const val DEFAULT_BACKEND_URL = "https://demo.rn.nella.org"
        
        fun getBackendUrl(context: Context): String {
            val prefs = context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)
            return prefs.getString(PREF_BACKEND_URL, DEFAULT_BACKEND_URL) ?: DEFAULT_BACKEND_URL
        }
        
        fun setBackendUrl(context: Context, url: String) {
            val prefs = context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)
            prefs.edit().putString(PREF_BACKEND_URL, url).apply()
        }
    }
    
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_settings)
        
        // Initialize views
        urlEditText = findViewById(R.id.urlEditText)
        saveButton = findViewById(R.id.saveButton)
        resetButton = findViewById(R.id.resetButton)
        
        // Initialize preferences
        prefs = getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)
        
        // Load current URL
        urlEditText.setText(getBackendUrl(this))
        
        // Set up click listeners
        saveButton.setOnClickListener {
            saveUrl()
        }
        
        resetButton.setOnClickListener {
            resetToDefault()
        }
        
        // Enable back button
        supportActionBar?.setDisplayHomeAsUpEnabled(true)
        supportActionBar?.title = "Settings"
    }
    
    private fun saveUrl() {
        val url = urlEditText.text.toString().trim()
        
        if (url.isEmpty()) {
            Toast.makeText(this, "URL cannot be empty", Toast.LENGTH_SHORT).show()
            return
        }
        
        if (!isValidUrl(url)) {
            Toast.makeText(this, "Please enter a valid URL (e.g., https://example.com)", Toast.LENGTH_LONG).show()
            return
        }
        
        // Remove trailing slash for consistency
        val cleanUrl = url.trimEnd('/')
        
        setBackendUrl(this, cleanUrl)
        Toast.makeText(this, "Backend URL saved successfully", Toast.LENGTH_SHORT).show()
        
        // Return to main activity
        finish()
    }
    
    private fun resetToDefault() {
        urlEditText.setText(DEFAULT_BACKEND_URL)
        Toast.makeText(this, "Reset to default URL", Toast.LENGTH_SHORT).show()
    }
    
    private fun isValidUrl(url: String): Boolean {
        return Patterns.WEB_URL.matcher(url).matches() && 
               (url.startsWith("http://") || url.startsWith("https://"))
    }
    
    override fun onSupportNavigateUp(): Boolean {
        finish()
        return true
    }
}
