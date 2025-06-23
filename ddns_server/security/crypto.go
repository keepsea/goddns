// ===================================================================================
// File: ddns-server/security/crypto.go
// Description: 提供了 Encrypt 和 Decrypt 两个核心函数，基于AES-GCM算法实现应用层的对称加密，确保了通信内容的机密性。
// ===================================================================================
package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
)

func Encrypt(key, plaintext []byte) (string, error) {
	if len(key) != 32 {
		return "", fmt.Errorf("密钥长度必须为16、24或32字节")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}
func Decrypt(key []byte, ciphertext string) ([]byte, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("密钥长度必须为16、24或32字节")
	}
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(data) < gcm.NonceSize() {
		return nil, fmt.Errorf("密文过短")
	}
	nonce, encryptedMessage := data[:gcm.NonceSize()], data[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, encryptedMessage, nil)
	if err != nil {
		return nil, err
	}
	return plaintext, nil
}
