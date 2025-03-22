package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/cloudflare/circl/kem"
	"github.com/cloudflare/circl/kem/kyber/kyber512"
)

// GenerateKyber512Keys creates a Kyber512 key pair.
func GenerateKyber512Keys() ([]byte, []byte, error) {
	// Use Kyber512 from Cloudflare's CIRCL library
	scheme := kyber512.Scheme()

	// Generate a new key pair
	publicKey, privateKey, err := scheme.GenerateKeyPair()
	if err != nil {
		return nil, nil, fmt.Errorf("Kyber512 keypair generation failed: %v", err)
	}

	// Convert to byte slices
	publicKeyBytes, err := publicKey.(kem.PublicKey).MarshalBinary()
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to marshal public key: %v", err)
	}

	privateKeyBytes, err := privateKey.(kem.PrivateKey).MarshalBinary()
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to marshal private key: %v", err)
	}

	return publicKeyBytes, privateKeyBytes, nil
}

// EncryptPrivateKey encrypts a private key using AES-GCM and returns a Base64 string.
func EncryptPrivateKey(privateKey []byte) (string, error) {
	fmt.Println("🔹 EncryptPrivateKey: Started encryption process")

	// 32-byte AES key (AES-256)
	aesKey := []byte("12345678901234567890123456789012")

	if len(aesKey) != 32 {
		return "", errors.New("AES key must be exactly 32 bytes")
	}
	if len(privateKey) == 0 {
		return "", errors.New("Private key is empty")
	}

	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return "", fmt.Errorf("AES cipher creation failed: %v", err)
	}

	nonce := make([]byte, 12)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("Nonce generation failed: %v", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("GCM mode initialization failed: %v", err)
	}

	// Encrypt private key.
	ciphertext := aesGCM.Seal(nil, nonce, privateKey, nil)
	finalCiphertext := append(nonce, ciphertext...)

	return base64.StdEncoding.EncodeToString(finalCiphertext), nil
}

// EncryptWithKyber encrypts a message using Kyber KEM.
func EncryptWithKyber(recipientPublicKeyBytes []byte) ([]byte, []byte, error) {
	// Use Kyber512 from Cloudflare's CIRCL library
	scheme := kyber512.Scheme()

	// Parse the recipient's public key
	publicKey, err := scheme.UnmarshalBinaryPublicKey(recipientPublicKeyBytes)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to unmarshal public key: %v", err)
	}

	// Generate a shared secret and ciphertext
	ciphertext, sharedSecret, err := scheme.Encapsulate(publicKey)
	if err != nil {
		return nil, nil, fmt.Errorf("Kyber encapsulation failed: %v", err)
	}

	return ciphertext, sharedSecret, nil
}

// DecryptWithKyber decrypts a ciphertext using Kyber KEM.
func DecryptWithKyber(privateKeyBytes, ciphertextBytes []byte) ([]byte, error) {
	// Use Kyber512 from Cloudflare's CIRCL library
	scheme := kyber512.Scheme()

	// Parse the private key
	privateKey, err := scheme.UnmarshalBinaryPrivateKey(privateKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("Failed to unmarshal private key: %v", err)
	}

	// Decrypt the ciphertext to get the shared secret
	sharedSecret, err := scheme.Decapsulate(privateKey, ciphertextBytes)
	if err != nil {
		return nil, fmt.Errorf("Kyber decapsulation failed: %v", err)
	}

	return sharedSecret, nil
}

// DecryptPrivateKey is not used server-side in this minimal example.
func DecryptPrivateKey(encryptedPrivateKey string) ([]byte, error) {
	return nil, errors.New("Server-side decryption not implemented")
}
