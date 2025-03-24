package aescrypt

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha1"
	"io"
	"strings"

	"golang.org/x/crypto/pbkdf2"
)

const (
	Iterations = 20000 // Iterations is the number of iterations used for key derivation
	KeySize    = 32    // KeySize is the size of the key used for encryption and decryption (32=AES-256)
)

func Encrypt(plaintext, password []byte) (ciphertext []byte, err error) {
	iv := make([]byte, aes.BlockSize)
	if _, err = io.ReadFull(rand.Reader, iv); err != nil {
		return
	}
	dk := pbkdf2.Key(password, iv, Iterations, KeySize, sha1.New)
	block, err := aes.NewCipher(dk)
	if err != nil {
		return
	}
	aescbc := cipher.NewCBCEncrypter(block, iv)
	ciphertext = make([]byte, len(plaintext))
	aescbc.CryptBlocks(ciphertext, plaintext)
	ciphertext = append(iv, ciphertext...)
	return
}

func Decrypt(ciphertext, password []byte) (plaintext []byte, err error) {
	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]
	dk := pbkdf2.Key(password, iv, Iterations, KeySize, sha1.New)
	block, err := aes.NewCipher(dk)
	if err != nil {
		return
	}
	aescbc := cipher.NewCBCDecrypter(block, iv)
	plaintext = make([]byte, len(ciphertext))
	aescbc.CryptBlocks(plaintext, ciphertext)
	return
}

func EncryptString(plaintext, password string) (ciphertext string, err error) {
	b := append([]byte(plaintext), make([]byte, aes.BlockSize-len(plaintext)%aes.BlockSize)...)
	c, err := Encrypt(b, []byte(password))
	ciphertext = string(c)
	return
}

func DecryptString(ciphertext, password string) (plaintext string, err error) {
	b, err := Decrypt([]byte(ciphertext), []byte(password))
	if err != nil {
		return
	}
	plaintext = strings.TrimRight(string(b), string("\x00"))
	return
}
