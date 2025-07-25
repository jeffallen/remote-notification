package org.nella.rn.demo

import android.util.Base64
import org.junit.Test
import java.security.KeyFactory
import java.security.PublicKey
import java.security.SecureRandom
import java.security.spec.X509EncodedKeySpec
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec
import javax.crypto.spec.SecretKeySpec

/**
 * Unit test to debug the token encryption issue between Android and Go backend.
 * This test shows exactly the components used in encryption so they can be verified
 * against the Go decryption implementation.
 */
class EncryptionDebugTest {

    companion object {
        // Public key from the project (same as used in production)
        private const val PUBLIC_KEY_PEM = """
-----BEGIN PUBLIC KEY-----
MIICIjANBgkqhkiG9w0BAQEFAAOCAg8AMIICCgKCAgEA2CTtuaZCrxUQqoEY4s8p
mqOUMICxIoOulUMdnj7vcfgBM2aksKR4/qXRwBDqAJNBuFfgTH+Lhv8LrUGd+eDS
jcng1tuqf5R/DEuADa+cXnhzQ8TCJL9sdwUJqzasN2TiZ0U8fZ+cyfF/HC3K1Ew5
wK++MAFZwJiiXVKEn4+xq7vAnW5kU2pywtLGghigbTB0vJGVaotXseh5Ufa0x50O
qdRQ63SFuNxLwQOQdyzpFw3AOzqL1hrYQ9MUriLbZ40o25WLkWoi74bX9JxABi84
ZduPe4RLNoRKZbRQFJAybC+7Vh1fJExCPsJHx74zDKX34TacxTyMp27LUt2dSHiu
BIS7Y7HSTEbg+G5eKGpVA5Rtl1YjNyc88LlZLr1671AnhimnmjuzL6FEWi+zn5Pa
+rH01JovEeOpNUFvlrggPIc6OX4/63yTkat0byuY2njhfpWyYWLnC1Ktmj5vanUZ
ZIGflsyrr/jCZVNopaAZ+MPqTkYcA6P+/XO6Ns8tLyO+i1XRLpaUD9XmwpjD/2W9
KSuoymhJiJ7hnb9hMmqcRERVAEyxI1McqkGbBFPOOgZv3mNZIOavkUdYP4yLOzAt
jQsmI58NsJ/YETayh+UN3mz1Z6jhysHodwrxyKzfPMnlJi9Ye0wRfSK3D4lXa8Ku
7Iho33GPlIheNdSv/eOGfysCAwEAAQ==
-----END PUBLIC KEY-----
        """.trimIndent()
    }

    @Test
    fun testEncryptionWithKnownValues() {
        println("=== Token Encryption Debug Test ===")
        
        // Use a known, predictable token for debugging
        val plaintextToken = "test_fcm_token_for_debugging_123456789"
        println("1. Plaintext Token: $plaintextToken")
        println("   Token Length: ${plaintextToken.length} characters")
        
        // Use fixed values for debugging (normally these would be random)
        val fixedAesKey = ByteArray(32) { (it % 256).toByte() } // Predictable pattern
        val fixedIv = ByteArray(12) { ((it * 17) % 256).toByte() } // Predictable pattern
        
        println("2. Fixed AES Key (32 bytes): ${bytesToHex(fixedAesKey)}")
        println("3. Fixed IV (12 bytes): ${bytesToHex(fixedIv)}")
        
        try {
            // Create AES key from fixed bytes
            val aesKey: SecretKey = SecretKeySpec(fixedAesKey, "AES")
            
            // Encrypt the token with AES-GCM
            val aesCipher = Cipher.getInstance("AES/GCM/NoPadding")
            val gcmSpec = GCMParameterSpec(128, fixedIv) // 128-bit authentication tag
            aesCipher.init(Cipher.ENCRYPT_MODE, aesKey, gcmSpec)
            
            val encryptedToken = aesCipher.doFinal(plaintextToken.toByteArray())
            println("4. AES-GCM Encrypted Token (${encryptedToken.size} bytes): ${bytesToHex(encryptedToken)}")
            
            // Load public key and encrypt the AES key with RSA
            val publicKey = loadPublicKeyFromPem(PUBLIC_KEY_PEM)
            val rsaCipher = Cipher.getInstance("RSA/ECB/PKCS1Padding")
            rsaCipher.init(Cipher.ENCRYPT_MODE, publicKey)
            val encryptedAesKey = rsaCipher.doFinal(fixedAesKey)
            
            println("5. RSA Encrypted AES Key (${encryptedAesKey.size} bytes): ${bytesToHex(encryptedAesKey)}")
            
            // Combine: IV (12 bytes) + encrypted AES key length (4 bytes) + encrypted AES key + encrypted token
            val keyLengthBytes = ByteArray(4)
            keyLengthBytes[0] = (encryptedAesKey.size shr 24).toByte()
            keyLengthBytes[1] = (encryptedAesKey.size shr 16).toByte()
            keyLengthBytes[2] = (encryptedAesKey.size shr 8).toByte()
            keyLengthBytes[3] = encryptedAesKey.size.toByte()
            
            println("6. Key Length Bytes (4 bytes): ${bytesToHex(keyLengthBytes)} (represents ${encryptedAesKey.size})")
            
            val combined = fixedIv + keyLengthBytes + encryptedAesKey + encryptedToken
            println("7. Combined Data (${combined.size} bytes): ${bytesToHex(combined)}")
            
            val finalEncryptedData = Base64.encodeToString(combined, Base64.DEFAULT).trim()
            println("8. Final Base64 Encrypted Data: $finalEncryptedData")
            
            println("\n=== Summary for Go Backend Testing ===")
            println("Plaintext Token: $plaintextToken")
            println("AES Key (hex): ${bytesToHex(fixedAesKey)}")
            println("IV (hex): ${bytesToHex(fixedIv)}")
            println("Final Encrypted Token: $finalEncryptedData")
            
            // Verify the format by parsing it back
            println("\n=== Verification ===")
            val decodedCombined = Base64.decode(finalEncryptedData, Base64.DEFAULT)
            println("Decoded combined length: ${decodedCombined.size} bytes")
            
            val parsedIv = decodedCombined.sliceArray(0..11)
            val parsedKeyLengthBytes = decodedCombined.sliceArray(12..15)
            val parsedKeyLength = (
                (parsedKeyLengthBytes[0].toInt() and 0xFF) shl 24 or
                (parsedKeyLengthBytes[1].toInt() and 0xFF) shl 16 or
                (parsedKeyLengthBytes[2].toInt() and 0xFF) shl 8 or
                (parsedKeyLengthBytes[3].toInt() and 0xFF)
            )
            
            println("Parsed IV matches: ${parsedIv.contentEquals(fixedIv)}")
            println("Parsed key length: $parsedKeyLength (expected: ${encryptedAesKey.size})")
            println("Key length matches: ${parsedKeyLength == encryptedAesKey.size}")
            
            if (16 + parsedKeyLength <= decodedCombined.size) {
                val parsedEncryptedAesKey = decodedCombined.sliceArray(16 until 16 + parsedKeyLength)
                val parsedEncryptedToken = decodedCombined.sliceArray(16 + parsedKeyLength until decodedCombined.size)
                
                println("Parsed encrypted AES key matches: ${parsedEncryptedAesKey.contentEquals(encryptedAesKey)}")
                println("Parsed encrypted token matches: ${parsedEncryptedToken.contentEquals(encryptedToken)}")
                println("Encrypted token length: ${parsedEncryptedToken.size} bytes")
            } else {
                println("ERROR: Combined data is too short for the expected key length")
            }
            
        } catch (e: Exception) {
            println("ERROR during encryption: ${e.message}")
            e.printStackTrace()
        }
    }
    
    @Test
    fun testEncryptionWithRandomValues() {
        println("\n=== Token Encryption with Random Values (Production-like) ===")
        
        val plaintextToken = "production_like_fcm_token_dGVzdF90b2tlbl9mb3JfZmNt:APA91bEhY1"
        println("1. Plaintext Token: $plaintextToken")
        
        try {
            // Generate random AES-256 key
            val keyGen = KeyGenerator.getInstance("AES")
            keyGen.init(256)
            val aesKey = keyGen.generateKey()
            
            // Generate random IV
            val iv = ByteArray(12)
            SecureRandom().nextBytes(iv)
            
            println("2. Random AES Key (32 bytes): ${bytesToHex(aesKey.encoded)}")
            println("3. Random IV (12 bytes): ${bytesToHex(iv)}")
            
            // Encrypt the token with AES-GCM
            val aesCipher = Cipher.getInstance("AES/GCM/NoPadding")
            val gcmSpec = GCMParameterSpec(128, iv)
            aesCipher.init(Cipher.ENCRYPT_MODE, aesKey, gcmSpec)
            
            val encryptedToken = aesCipher.doFinal(plaintextToken.toByteArray())
            println("4. AES-GCM Encrypted Token (${encryptedToken.size} bytes): ${bytesToHex(encryptedToken)}")
            
            // Load public key and encrypt the AES key with RSA
            val publicKey = loadPublicKeyFromPem(PUBLIC_KEY_PEM)
            val rsaCipher = Cipher.getInstance("RSA/ECB/PKCS1Padding")
            rsaCipher.init(Cipher.ENCRYPT_MODE, publicKey)
            val encryptedAesKey = rsaCipher.doFinal(aesKey.encoded)
            
            println("5. RSA Encrypted AES Key (${encryptedAesKey.size} bytes): ${bytesToHex(encryptedAesKey)}")
            
            // Combine all parts
            val keyLengthBytes = ByteArray(4)
            keyLengthBytes[0] = (encryptedAesKey.size shr 24).toByte()
            keyLengthBytes[1] = (encryptedAesKey.size shr 16).toByte()
            keyLengthBytes[2] = (encryptedAesKey.size shr 8).toByte()
            keyLengthBytes[3] = encryptedAesKey.size.toByte()
            
            val combined = iv + keyLengthBytes + encryptedAesKey + encryptedToken
            val finalEncryptedData = Base64.encodeToString(combined, Base64.DEFAULT).trim()
            
            println("6. Final Base64 Encrypted Data: $finalEncryptedData")
            
            println("\n=== Random Values Summary ===")
            println("This shows the format is consistent even with random values.")
            println("Encrypted data length: ${finalEncryptedData.length} characters")
            
        } catch (e: Exception) {
            println("ERROR during random encryption: ${e.message}")
            e.printStackTrace()
        }
    }
    
    private fun loadPublicKeyFromPem(publicKeyPem: String): PublicKey {
        // Remove PEM headers and footers, and newlines
        val publicKeyBase64 = publicKeyPem
            .replace("-----BEGIN PUBLIC KEY-----", "")
            .replace("-----END PUBLIC KEY-----", "")
            .replace("\n", "")
            .replace("\r", "")
            .trim()
        
        val keyBytes = Base64.decode(publicKeyBase64, Base64.DEFAULT)
        val keySpec = X509EncodedKeySpec(keyBytes)
        val keyFactory = KeyFactory.getInstance("RSA")
        
        return keyFactory.generatePublic(keySpec)
    }
    
    private fun bytesToHex(bytes: ByteArray): String {
        return bytes.joinToString("") { "%02x".format(it) }
    }
}
