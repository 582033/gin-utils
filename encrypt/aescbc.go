package encrypt

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
)

func PKCS7Padding(cipherText []byte, blockSize int) []byte {
	padding := blockSize - len(cipherText)%blockSize
	paddingText := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(cipherText, paddingText...)
}

func PKCS7UnPadding(origData []byte) []byte {
	length := len(origData)
	unPadding := int(origData[length-1])
	return origData[:(length - unPadding)]
}

//填充秘钥key的16位，24,32分别对应AES-128, AES-192, or AES-256.
func aesCBCEncrypt(rawData, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	blockSize := block.BlockSize()
	rawData = PKCS7Padding(rawData, blockSize)
	cipherText := make([]byte, blockSize+len(rawData))
	iv := cipherText[:blockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}

	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(cipherText[blockSize:], rawData)

	return cipherText, nil
}

func aesCBCDncrypt(encryptData, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	blockSize := block.BlockSize()
	if len(encryptData) < blockSize {
		return nil, errors.New("ciphertext too short")
	}

	iv := encryptData[:blockSize]
	encryptData = encryptData[blockSize:]

	if len(encryptData)%blockSize != 0 {
		return nil, errors.New("ciphertext is not a multiple of the block size")
	}

	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(encryptData, encryptData)

	encryptData = PKCS7UnPadding(encryptData)
	return encryptData, nil
}

func AesCBCEncrypt(rawData, key []byte) (bData []byte, err1 error) {
	defer func() {
		if err := recover(); err != nil {
			err1 = err.(error)
		}
	}()
	src, err := aesCBCEncrypt(rawData, key)
	if err != nil {
		return nil, err
	}
	buf := make([]byte, base64.StdEncoding.EncodedLen(len(src)))
	base64.StdEncoding.Encode(buf, src)
	return buf, nil
}

func AesCBCDncrypt(rawData, key []byte) (data1 []byte, err1 error) {
	defer func() {
		if err := recover(); err != nil {
			err1 = err.(error)
		}
	}()
	buf := make([]byte, base64.StdEncoding.DecodedLen(len(rawData)))
	n, err := base64.StdEncoding.Decode(buf, []byte(rawData))
	if err != nil {
		return nil, err
	}
	data := buf[:n]
	dnData, err := aesCBCDncrypt(data, key)
	if err != nil {
		return nil, err
	}
	return dnData, nil
}
