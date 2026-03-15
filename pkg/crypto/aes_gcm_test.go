package crypto

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testKey() []byte {
	key := make([]byte, 32)
	_, _ = rand.Read(key)
	return key
}

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	key := testKey()
	plaintext := []byte("apiVersion: v1\nkind: Config\nclusters:\n- name: test")

	encrypted, err := Encrypt(key, plaintext)
	require.NoError(t, err)
	assert.NotEmpty(t, encrypted)
	assert.NotEqual(t, string(plaintext), encrypted)

	decrypted, err := Decrypt(key, encrypted)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestEncrypt_DifferentNonceEachTime(t *testing.T) {
	key := testKey()
	plaintext := []byte("same data")

	enc1, err := Encrypt(key, plaintext)
	require.NoError(t, err)

	enc2, err := Encrypt(key, plaintext)
	require.NoError(t, err)

	assert.NotEqual(t, enc1, enc2, "same plaintext should produce different ciphertext due to random nonce")
}

func TestDecrypt_WrongKey(t *testing.T) {
	key1 := testKey()
	key2 := testKey()
	plaintext := []byte("secret kubeconfig")

	encrypted, err := Encrypt(key1, plaintext)
	require.NoError(t, err)

	_, err = Decrypt(key2, encrypted)
	assert.Error(t, err)
}

func TestEncrypt_InvalidKeyLength(t *testing.T) {
	_, err := Encrypt([]byte("short"), []byte("data"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "32 bytes")
}

func TestDecrypt_InvalidBase64(t *testing.T) {
	key := testKey()
	_, err := Decrypt(key, "not-valid-base64!!!")
	assert.Error(t, err)
}

func TestDecrypt_TamperedCiphertext(t *testing.T) {
	key := testKey()
	encrypted, err := Encrypt(key, []byte("original"))
	require.NoError(t, err)

	tampered := encrypted[:len(encrypted)-2] + "XX"
	_, err = Decrypt(key, tampered)
	assert.Error(t, err)
}
