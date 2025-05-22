package vm

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io"
)

type CryptoModule struct {
	vm *VM
}

func NewCryptoModule(vm *VM) *CryptoModule {
	return &CryptoModule{vm: vm}
}

func (cm *CryptoModule) RegisterFunctions() {
	cm.vm.RegisterFunction("sha256", []Instruction{
		{Op: PUSH, Value: 0},
		{Op: LOAD, Value: 0},
		{Op: CALL, Value: "__crypto_sha256"},
		{Op: RET, Value: nil},
	})

	cm.vm.RegisterFunction("aes_encrypt", []Instruction{
		{Op: PUSH, Value: 0},
		{Op: LOAD, Value: 0},
		{Op: PUSH, Value: 1},
		{Op: LOAD, Value: 1},
		{Op: CALL, Value: "__crypto_aes_encrypt"},
		{Op: RET, Value: nil},
	})

	cm.vm.RegisterFunction("aes_decrypt", []Instruction{
		{Op: PUSH, Value: 0},
		{Op: LOAD, Value: 0},
		{Op: PUSH, Value: 1},
		{Op: LOAD, Value: 1},
		{Op: CALL, Value: "__crypto_aes_decrypt"},
		{Op: RET, Value: nil},
	})

	cm.vm.RegisterFunction("rsa_generate", []Instruction{
		{Op: PUSH, Value: 0},
		{Op: LOAD, Value: 0},
		{Op: CALL, Value: "__crypto_rsa_generate"},
		{Op: RET, Value: nil},
	})

	cm.vm.RegisterFunction("rsa_encrypt", []Instruction{
		{Op: PUSH, Value: 0},
		{Op: LOAD, Value: 0},
		{Op: PUSH, Value: 1},
		{Op: LOAD, Value: 1},
		{Op: CALL, Value: "__crypto_rsa_encrypt"},
		{Op: RET, Value: nil},
	})

	cm.vm.RegisterFunction("rsa_decrypt", []Instruction{
		{Op: PUSH, Value: 0},
		{Op: LOAD, Value: 0},
		{Op: PUSH, Value: 1},
		{Op: LOAD, Value: 1},
		{Op: CALL, Value: "__crypto_rsa_decrypt"},
		{Op: RET, Value: nil},
	})
}

func (cm *CryptoModule) SHA256(data []byte) []byte {
	hash := sha256.Sum256(data)
	return hash[:]
}

func (cm *CryptoModule) AESEncrypt(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	ciphertext := make([]byte, aes.BlockSize+len(plaintext))
	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}

	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], plaintext)

	return ciphertext, nil
}

func (cm *CryptoModule) AESDecrypt(key, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	if len(ciphertext) < aes.BlockSize {
		return nil, errors.New("ciphertext too short")
	}

	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]

	stream := cipher.NewCFBDecrypter(block, iv)
	stream.XORKeyStream(ciphertext, ciphertext)

	return ciphertext, nil
}

func (cm *CryptoModule) RSAGenerate(bits int) (*rsa.PrivateKey, error) {
	return rsa.GenerateKey(rand.Reader, bits)
}

func (cm *CryptoModule) RSAEncrypt(publicKey *rsa.PublicKey, plaintext []byte) ([]byte, error) {
	return rsa.EncryptPKCS1v15(rand.Reader, publicKey, plaintext)
}

func (cm *CryptoModule) RSADecrypt(privateKey *rsa.PrivateKey, ciphertext []byte) ([]byte, error) {
	return rsa.DecryptPKCS1v15(rand.Reader, privateKey, ciphertext)
}

func (cm *CryptoModule) ExportRSAPublicKey(key *rsa.PublicKey) (string, error) {
	der, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		return "", err
	}

	block := &pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: der,
	}

	return string(pem.EncodeToMemory(block)), nil
}

func (cm *CryptoModule) ExportRSAPrivateKey(key *rsa.PrivateKey) (string, error) {
	der := x509.MarshalPKCS1PrivateKey(key)
	block := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: der,
	}

	return string(pem.EncodeToMemory(block)), nil
}

func (cm *CryptoModule) ImportRSAPublicKey(pemStr string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, errors.New("failed to parse PEM block")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	return pub.(*rsa.PublicKey), nil
}

func (cm *CryptoModule) ImportRSAPrivateKey(pemStr string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, errors.New("failed to parse PEM block")
	}

	return x509.ParsePKCS1PrivateKey(block.Bytes)
}
