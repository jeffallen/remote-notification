import java.security.KeyFactory;
import java.security.PublicKey;
import java.security.SecureRandom;
import java.security.spec.X509EncodedKeySpec;
import java.util.Base64;
import javax.crypto.Cipher;
import javax.crypto.KeyGenerator;
import javax.crypto.SecretKey;
import javax.crypto.spec.GCMParameterSpec;
import javax.crypto.spec.SecretKeySpec;

/**
 * Standalone Java test to debug the token encryption issue between Android and Go backend.
 * This test shows exactly the components used in encryption so they can be verified
 * against the Go decryption implementation.
 */
public class EncryptionDebugTest {

    // Public key from the project (same as used in production)
    private static final String PUBLIC_KEY_PEM = 
        "-----BEGIN PUBLIC KEY-----\n" +
        "MIICIjANBgkqhkiG9w0BAQEFAAOCAg8AMIICCgKCAgEA2CTtuaZCrxUQqoEY4s8p\n" +
        "mqOUMICxIoOulUMdnj7vcfgBM2aksKR4/qXRwBDqAJNBuFfgTH+Lhv8LrUGd+eDS\n" +
        "jcng1tuqf5R/DEuADa+cXnhzQ8TCJL9sdwUJqzasN2TiZ0U8fZ+cyfF/HC3K1Ew5\n" +
        "wK++MAFZwJiiXVKEn4+xq7vAnW5kU2pywtLGghigbTB0vJGVaotXseh5Ufa0x50O\n" +
        "qdRQ63SFuNxLwQOQdyzpFw3AOzqL1hrYQ9MUriLbZ40o25WLkWoi74bX9JxABi84\n" +
        "ZduPe4RLNoRKZbRQFJAybC+7Vh1fJExCPsJHx74zDKX34TacxTyMp27LUt2dSHiu\n" +
        "BIS7Y7HSTEbg+G5eKGpVA5Rtl1YjNyc88LlZLr1671AnhimnmjuzL6FEWi+zn5Pa\n" +
        "+rH01JovEeOpNUFvlrggPIc6OX4/63yTkat0byuY2njhfpWyYWLnC1Ktmj5vanUZ\n" +
        "ZIGflsyrr/jCZVNopaAZ+MPqTkYcA6P+/XO6Ns8tLyO+i1XRLpaUD9XmwpjD/2W9\n" +
        "KSuoymhJiJ7hnb9hMmqcRERVAEyxI1McqkGbBFPOOgZv3mNZIOavkUdYP4yLOzAt\n" +
        "jQsmI58NsJ/YETayh+UN3mz1Z6jhysHodwrxyKzfPMnlJi9Ye0wRfSK3D4lXa8Ku\n" +
        "7Iho33GPlIheNdSv/eOGfysCAwEAAQ==\n" +
        "-----END PUBLIC KEY-----";

    public static void main(String[] args) {
        try {
            testEncryptionWithKnownValues();
            System.out.println();
            testEncryptionWithRandomValues();
        } catch (Exception e) {
            e.printStackTrace();
        }
    }

    public static void testEncryptionWithKnownValues() throws Exception {
        System.out.println("=== Token Encryption Debug Test ===");
        
        // Use a known, predictable token for debugging
        String plaintextToken = "test_fcm_token_for_debugging_123456789";
        System.out.println("1. Plaintext Token: " + plaintextToken);
        System.out.println("   Token Length: " + plaintextToken.length() + " characters");
        
        // Use fixed values for debugging (normally these would be random)
        byte[] fixedAesKey = new byte[32];
        for (int i = 0; i < 32; i++) {
            fixedAesKey[i] = (byte)(i % 256);
        }
        
        byte[] fixedIv = new byte[12];
        for (int i = 0; i < 12; i++) {
            fixedIv[i] = (byte)((i * 17) % 256);
        }
        
        System.out.println("2. Fixed AES Key (32 bytes): " + bytesToHex(fixedAesKey));
        System.out.println("3. Fixed IV (12 bytes): " + bytesToHex(fixedIv));
        
        // Create AES key from fixed bytes
        SecretKey aesKey = new SecretKeySpec(fixedAesKey, "AES");
        
        // Encrypt the token with AES-GCM
        Cipher aesCipher = Cipher.getInstance("AES/GCM/NoPadding");
        GCMParameterSpec gcmSpec = new GCMParameterSpec(128, fixedIv); // 128-bit authentication tag
        aesCipher.init(Cipher.ENCRYPT_MODE, aesKey, gcmSpec);
        
        byte[] encryptedToken = aesCipher.doFinal(plaintextToken.getBytes());
        System.out.println("4. AES-GCM Encrypted Token (" + encryptedToken.length + " bytes): " + bytesToHex(encryptedToken));
        
        // Load public key and encrypt the AES key with RSA
        PublicKey publicKey = loadPublicKeyFromPem(PUBLIC_KEY_PEM);
        Cipher rsaCipher = Cipher.getInstance("RSA/ECB/PKCS1Padding");
        rsaCipher.init(Cipher.ENCRYPT_MODE, publicKey);
        byte[] encryptedAESKey = rsaCipher.doFinal(fixedAesKey);
        
        System.out.println("5. RSA Encrypted AES Key (" + encryptedAESKey.length + " bytes): " + bytesToHex(encryptedAESKey));
        
        // Combine: IV (12 bytes) + encrypted AES key length (4 bytes) + encrypted AES key + encrypted token
        byte[] keyLengthBytes = new byte[4];
        keyLengthBytes[0] = (byte)(encryptedAESKey.length >> 24);
        keyLengthBytes[1] = (byte)(encryptedAESKey.length >> 16);
        keyLengthBytes[2] = (byte)(encryptedAESKey.length >> 8);
        keyLengthBytes[3] = (byte)encryptedAESKey.length;
        
        System.out.println("6. Key Length Bytes (4 bytes): " + bytesToHex(keyLengthBytes) + " (represents " + encryptedAESKey.length + ")");
        
        byte[] combined = new byte[fixedIv.length + keyLengthBytes.length + encryptedAESKey.length + encryptedToken.length];
        int pos = 0;
        System.arraycopy(fixedIv, 0, combined, pos, fixedIv.length);
        pos += fixedIv.length;
        System.arraycopy(keyLengthBytes, 0, combined, pos, keyLengthBytes.length);
        pos += keyLengthBytes.length;
        System.arraycopy(encryptedAESKey, 0, combined, pos, encryptedAESKey.length);
        pos += encryptedAESKey.length;
        System.arraycopy(encryptedToken, 0, combined, pos, encryptedToken.length);
        
        System.out.println("7. Combined Data (" + combined.length + " bytes): " + bytesToHex(combined));
        
        String finalEncryptedData = Base64.getEncoder().encodeToString(combined);
        System.out.println("8. Final Base64 Encrypted Data: " + finalEncryptedData);
        
        System.out.println();
        System.out.println("=== Summary for Go Backend Testing ===");
        System.out.println("Plaintext Token: " + plaintextToken);
        System.out.println("AES Key (hex): " + bytesToHex(fixedAesKey));
        System.out.println("IV (hex): " + bytesToHex(fixedIv));
        System.out.println("Final Encrypted Token: " + finalEncryptedData);
        
        // Verify the format by parsing it back
        System.out.println();
        System.out.println("=== Verification ===");
        byte[] decodedCombined = Base64.getDecoder().decode(finalEncryptedData);
        System.out.println("Decoded combined length: " + decodedCombined.length + " bytes");
        
        byte[] parsedIv = new byte[12];
        System.arraycopy(decodedCombined, 0, parsedIv, 0, 12);
        
        byte[] parsedKeyLengthBytes = new byte[4];
        System.arraycopy(decodedCombined, 12, parsedKeyLengthBytes, 0, 4);
        
        int parsedKeyLength = 
            ((parsedKeyLengthBytes[0] & 0xFF) << 24) |
            ((parsedKeyLengthBytes[1] & 0xFF) << 16) |
            ((parsedKeyLengthBytes[2] & 0xFF) << 8) |
            (parsedKeyLengthBytes[3] & 0xFF);
        
        System.out.println("Parsed IV matches: " + java.util.Arrays.equals(parsedIv, fixedIv));
        System.out.println("Parsed key length: " + parsedKeyLength + " (expected: " + encryptedAESKey.length + ")");
        System.out.println("Key length matches: " + (parsedKeyLength == encryptedAESKey.length));
        
        if (16 + parsedKeyLength <= decodedCombined.length) {
            byte[] parsedEncryptedAESKey = new byte[parsedKeyLength];
            System.arraycopy(decodedCombined, 16, parsedEncryptedAESKey, 0, parsedKeyLength);
            
            byte[] parsedEncryptedToken = new byte[decodedCombined.length - 16 - parsedKeyLength];
            System.arraycopy(decodedCombined, 16 + parsedKeyLength, parsedEncryptedToken, 0, parsedEncryptedToken.length);
            
            System.out.println("Parsed encrypted AES key matches: " + java.util.Arrays.equals(parsedEncryptedAESKey, encryptedAESKey));
            System.out.println("Parsed encrypted token matches: " + java.util.Arrays.equals(parsedEncryptedToken, encryptedToken));
            System.out.println("Encrypted token length: " + parsedEncryptedToken.length + " bytes");
        } else {
            System.out.println("ERROR: Combined data is too short for the expected key length");
        }
    }
    
    public static void testEncryptionWithRandomValues() throws Exception {
        System.out.println("=== Token Encryption with Random Values (Production-like) ===");
        
        String plaintextToken = "production_like_fcm_token_dGVzdF90b2tlbl9mb3JfZmNt:APA91bEhY1";
        System.out.println("1. Plaintext Token: " + plaintextToken);
        
        // Generate random AES-256 key
        KeyGenerator keyGen = KeyGenerator.getInstance("AES");
        keyGen.init(256);
        SecretKey aesKey = keyGen.generateKey();
        
        // Generate random IV
        byte[] iv = new byte[12];
        new SecureRandom().nextBytes(iv);
        
        System.out.println("2. Random AES Key (32 bytes): " + bytesToHex(aesKey.getEncoded()));
        System.out.println("3. Random IV (12 bytes): " + bytesToHex(iv));
        
        // Encrypt the token with AES-GCM
        Cipher aesCipher = Cipher.getInstance("AES/GCM/NoPadding");
        GCMParameterSpec gcmSpec = new GCMParameterSpec(128, iv);
        aesCipher.init(Cipher.ENCRYPT_MODE, aesKey, gcmSpec);
        
        byte[] encryptedToken = aesCipher.doFinal(plaintextToken.getBytes());
        System.out.println("4. AES-GCM Encrypted Token (" + encryptedToken.length + " bytes): " + bytesToHex(encryptedToken));
        
        // Load public key and encrypt the AES key with RSA
        PublicKey publicKey = loadPublicKeyFromPem(PUBLIC_KEY_PEM);
        Cipher rsaCipher = Cipher.getInstance("RSA/ECB/PKCS1Padding");
        rsaCipher.init(Cipher.ENCRYPT_MODE, publicKey);
        byte[] encryptedAESKey = rsaCipher.doFinal(aesKey.getEncoded());
        
        System.out.println("5. RSA Encrypted AES Key (" + encryptedAESKey.length + " bytes): " + bytesToHex(encryptedAESKey));
        
        // Combine all parts
        byte[] keyLengthBytes = new byte[4];
        keyLengthBytes[0] = (byte)(encryptedAESKey.length >> 24);
        keyLengthBytes[1] = (byte)(encryptedAESKey.length >> 16);
        keyLengthBytes[2] = (byte)(encryptedAESKey.length >> 8);
        keyLengthBytes[3] = (byte)encryptedAESKey.length;
        
        byte[] combined = new byte[iv.length + keyLengthBytes.length + encryptedAESKey.length + encryptedToken.length];
        int pos = 0;
        System.arraycopy(iv, 0, combined, pos, iv.length);
        pos += iv.length;
        System.arraycopy(keyLengthBytes, 0, combined, pos, keyLengthBytes.length);
        pos += keyLengthBytes.length;
        System.arraycopy(encryptedAESKey, 0, combined, pos, encryptedAESKey.length);
        pos += encryptedAESKey.length;
        System.arraycopy(encryptedToken, 0, combined, pos, encryptedToken.length);
        
        String finalEncryptedData = Base64.getEncoder().encodeToString(combined);
        
        System.out.println("6. Final Base64 Encrypted Data: " + finalEncryptedData);
        
        System.out.println();
        System.out.println("=== Random Values Summary ===");
        System.out.println("This shows the format is consistent even with random values.");
        System.out.println("Encrypted data length: " + finalEncryptedData.length() + " characters");
    }
    
    private static PublicKey loadPublicKeyFromPem(String publicKeyPem) throws Exception {
        // Remove PEM headers and footers, and newlines
        String publicKeyBase64 = publicKeyPem
            .replace("-----BEGIN PUBLIC KEY-----", "")
            .replace("-----END PUBLIC KEY-----", "")
            .replace("\n", "")
            .replace("\r", "")
            .trim();
        
        byte[] keyBytes = Base64.getDecoder().decode(publicKeyBase64);
        X509EncodedKeySpec keySpec = new X509EncodedKeySpec(keyBytes);
        KeyFactory keyFactory = KeyFactory.getInstance("RSA");
        
        return keyFactory.generatePublic(keySpec);
    }
    
    private static String bytesToHex(byte[] bytes) {
        StringBuilder sb = new StringBuilder();
        for (byte b : bytes) {
            sb.append(String.format("%02x", b));
        }
        return sb.toString();
    }
}
