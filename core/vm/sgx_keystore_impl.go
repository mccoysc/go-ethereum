// Copyright 2024 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package vm

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"golang.org/x/crypto/hkdf"
)

// zeroBytes securely zeros the given byte slice to prevent data leakage
func zeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

// EncryptedKeyStore 实现 KeyStore 接口，支持加密存储
type EncryptedKeyStore struct {
	encryptedPath string // 加密分区路径
	publicPath    string // 公开数据路径
}

// NewEncryptedKeyStore 创建新的密钥存储
func NewEncryptedKeyStore(encryptedPath, publicPath string) (*EncryptedKeyStore, error) {
	// 创建目录
	if err := os.MkdirAll(encryptedPath, 0700); err != nil {
		return nil, fmt.Errorf("failed to create encrypted directory: %w", err)
	}
	if err := os.MkdirAll(publicPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create public directory: %w", err)
	}
	
	return &EncryptedKeyStore{
		encryptedPath: encryptedPath,
		publicPath:    publicPath,
	}, nil
}

// CreateKey 创建新密钥
func (ks *EncryptedKeyStore) CreateKey(owner common.Address, keyType KeyType) (common.Hash, error) {
	var keyID common.Hash
	var pubKey []byte
	var privKey interface{}
	
	switch keyType {
	case KeyTypeECDSA:
		// 生成 secp256k1 密钥对
		privateKey, err := crypto.GenerateKey()
		if err != nil {
			return common.Hash{}, fmt.Errorf("failed to generate ECDSA key: %w", err)
		}
		privKey = privateKey
		pubKey = crypto.FromECDSAPub(&privateKey.PublicKey)
		keyID = crypto.Keccak256Hash(pubKey)
		
	case KeyTypeEd25519:
		// 生成 Ed25519 密钥对
		pubKeyEd, privKeyEd, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return common.Hash{}, fmt.Errorf("failed to generate Ed25519 key: %w", err)
		}
		privKey = privKeyEd
		pubKey = pubKeyEd
		keyID = crypto.Keccak256Hash(pubKey)
		
	case KeyTypeAES256:
		// 生成 AES-256 密钥
		aesKey := make([]byte, 32)
		if _, err := io.ReadFull(rand.Reader, aesKey); err != nil {
			return common.Hash{}, fmt.Errorf("failed to generate AES key: %w", err)
		}
		privKey = aesKey
		pubKey = aesKey // 对称密钥，公钥即私钥
		keyID = crypto.Keccak256Hash(aesKey)
		
	default:
		return common.Hash{}, fmt.Errorf("unsupported key type: %d", keyType)
	}
	
	// 保存私钥到加密分区
	if err := ks.savePrivateKey(keyID, privKey); err != nil {
		return common.Hash{}, err
	}
	
	// 保存元数据到公开分区
	metadata := &KeyMetadata{
		KeyID:       keyID,
		Owner:       owner,
		KeyType:     keyType,
		CreatedAt:   0, // 时间戳由调用者设置
		CreatedBy:   owner,
		Permissions: []Permission{},
	}
	if err := ks.saveMetadata(metadata); err != nil {
		return common.Hash{}, err
	}
	
	return keyID, nil
}

// GetPublicKey 获取公钥
func (ks *EncryptedKeyStore) GetPublicKey(keyID common.Hash) ([]byte, error) {
	metadata, err := ks.GetMetadata(keyID)
	if err != nil {
		return nil, err
	}
	
	privKey, err := ks.loadPrivateKey(keyID, metadata.KeyType)
	if err != nil {
		return nil, err
	}
	
	switch metadata.KeyType {
	case KeyTypeECDSA:
		ecdsaKey, ok := privKey.(*ecdsa.PrivateKey)
		if !ok {
			return nil, errors.New("invalid ECDSA key")
		}
		return crypto.FromECDSAPub(&ecdsaKey.PublicKey), nil
		
	case KeyTypeEd25519:
		ed25519Key, ok := privKey.(ed25519.PrivateKey)
		if !ok {
			return nil, errors.New("invalid Ed25519 key")
		}
		return []byte(ed25519Key.Public().(ed25519.PublicKey)), nil
		
	case KeyTypeAES256:
		// 对称密钥不公开
		return nil, errors.New("AES keys have no public component")
		
	default:
		return nil, fmt.Errorf("unsupported key type: %d", metadata.KeyType)
	}
}

// Sign 使用密钥签名
func (ks *EncryptedKeyStore) Sign(keyID common.Hash, hash []byte) ([]byte, error) {
	metadata, err := ks.GetMetadata(keyID)
	if err != nil {
		return nil, err
	}
	
	if metadata.KeyType != KeyTypeECDSA && metadata.KeyType != KeyTypeEd25519 {
		return nil, errors.New("key type does not support signing")
	}
	
	privKey, err := ks.loadPrivateKey(keyID, metadata.KeyType)
	if err != nil {
		return nil, err
	}
	
	switch metadata.KeyType {
	case KeyTypeECDSA:
		ecdsaKey, ok := privKey.(*ecdsa.PrivateKey)
		if !ok {
			return nil, errors.New("invalid ECDSA key")
		}
		// Ensure we zero the big.Int's internal bits after use
		defer func() {
			if ecdsaKey.D != nil {
				ecdsaKey.D.SetInt64(0)
			}
		}()
		signature, err := crypto.Sign(hash, ecdsaKey)
		if err != nil {
			return nil, fmt.Errorf("failed to sign: %w", err)
		}
		return signature, nil
		
	case KeyTypeEd25519:
		ed25519Key, ok := privKey.(ed25519.PrivateKey)
		if !ok {
			return nil, errors.New("invalid Ed25519 key")
		}
		signature := ed25519.Sign(ed25519Key, hash)
		// Zero the private key
		defer zeroBytes(ed25519Key)
		return signature, nil
		
	default:
		return nil, fmt.Errorf("unsupported signing key type: %d", metadata.KeyType)
	}
}

// ECDH 执行 ECDH 密钥交换，可选应用 KDF，返回新密钥 ID
func (ks *EncryptedKeyStore) ECDH(keyID common.Hash, peerPubKey []byte, kdfParams []byte) (common.Hash, error) {
	metadata, err := ks.GetMetadata(keyID)
	if err != nil {
		return common.Hash{}, err
	}
	
	if metadata.KeyType != KeyTypeECDSA {
		return common.Hash{}, errors.New("only ECDSA keys support ECDH")
	}
	
	privKey, err := ks.loadPrivateKey(keyID, metadata.KeyType)
	if err != nil {
		return common.Hash{}, err
	}
	
	ecdsaKey, ok := privKey.(*ecdsa.PrivateKey)
	if !ok {
		return common.Hash{}, errors.New("invalid ECDSA key")
	}
	
	// Zero the private key big.Int after use
	defer func() {
		if ecdsaKey.D != nil {
			ecdsaKey.D.SetInt64(0)
		}
	}()
	
	// 解析对方公钥
	peerPublicKey, err := crypto.UnmarshalPubkey(peerPubKey)
	if err != nil {
		return common.Hash{}, fmt.Errorf("invalid peer public key: %w", err)
	}
	
	// 执行 ECDH
	x, _ := peerPublicKey.Curve.ScalarMult(peerPublicKey.X, peerPublicKey.Y, ecdsaKey.D.Bytes())
	sharedSecret := crypto.Keccak256(x.Bytes())
	defer zeroBytes(sharedSecret)
	
	// Apply KDF if kdfParams provided
	var derivedKey []byte
	if len(kdfParams) > 0 {
		// Use HKDF with the shared secret
		hkdfReader := hkdf.New(sha256.New, sharedSecret, kdfParams, []byte("ecdh-shared-secret"))
		derivedKey = make([]byte, 32)
		if _, err := io.ReadFull(hkdfReader, derivedKey); err != nil {
			return common.Hash{}, fmt.Errorf("failed to derive key: %w", err)
		}
		defer zeroBytes(derivedKey)
	} else {
		// Use raw shared secret
		derivedKey = make([]byte, 32)
		copy(derivedKey, sharedSecret)
		defer zeroBytes(derivedKey)
	}
	
	// Create new AES256 key from the derived material
	newKeyID := crypto.Keccak256Hash(derivedKey)
	
	// Save the derived key as an AES256 key
	if err := ks.savePrivateKey(newKeyID, derivedKey); err != nil {
		return common.Hash{}, err
	}
	
	// Save metadata for the new key (owned by the caller)
	newMetadata := &KeyMetadata{
		KeyID:       newKeyID,
		Owner:       metadata.Owner,
		KeyType:     KeyTypeAES256,
		CreatedAt:   0,
		CreatedBy:   metadata.Owner,
		Permissions: []Permission{},
	}
	if err := ks.saveMetadata(newMetadata); err != nil {
		return common.Hash{}, err
	}
	
	return newKeyID, nil
}

// Encrypt 加密数据
func (ks *EncryptedKeyStore) Encrypt(keyID common.Hash, plaintext []byte) ([]byte, error) {
	metadata, err := ks.GetMetadata(keyID)
	if err != nil {
		return nil, err
	}
	
	if metadata.KeyType != KeyTypeAES256 {
		return nil, errors.New("only AES keys support encryption")
	}
	
	privKey, err := ks.loadPrivateKey(keyID, metadata.KeyType)
	if err != nil {
		return nil, err
	}
	
	aesKey, ok := privKey.([]byte)
	if !ok {
		return nil, errors.New("invalid AES key")
	}
	defer zeroBytes(aesKey)
	
	// 使用 AES-GCM 加密
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}
	
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}
	
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}
	
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt 解密数据
func (ks *EncryptedKeyStore) Decrypt(keyID common.Hash, ciphertext []byte) ([]byte, error) {
	metadata, err := ks.GetMetadata(keyID)
	if err != nil {
		return nil, err
	}
	
	if metadata.KeyType != KeyTypeAES256 {
		return nil, errors.New("only AES keys support decryption")
	}
	
	privKey, err := ks.loadPrivateKey(keyID, metadata.KeyType)
	if err != nil {
		return nil, err
	}
	
	aesKey, ok := privKey.([]byte)
	if !ok {
		return nil, errors.New("invalid AES key")
	}
	defer zeroBytes(aesKey)
	
	// 使用 AES-GCM 解密
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}
	
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}
	
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}
	
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}
	
	return plaintext, nil
}

// DeriveKey 派生子密钥
func (ks *EncryptedKeyStore) DeriveKey(keyID common.Hash, path []byte) (common.Hash, error) {
	metadata, err := ks.GetMetadata(keyID)
	if err != nil {
		return common.Hash{}, err
	}
	
	privKey, err := ks.loadPrivateKey(keyID, metadata.KeyType)
	if err != nil {
		return common.Hash{}, err
	}
	
	var derivedKey []byte
	
	switch metadata.KeyType {
	case KeyTypeECDSA:
		ecdsaKey, ok := privKey.(*ecdsa.PrivateKey)
		if !ok {
			return common.Hash{}, errors.New("invalid ECDSA key")
		}
		// 使用 HKDF 派生
		derivedKey = crypto.Keccak256(ecdsaKey.D.Bytes(), path)
		// Zero the big.Int after use
		defer func() {
			if ecdsaKey.D != nil {
				ecdsaKey.D.SetInt64(0)
			}
		}()
		
	case KeyTypeEd25519:
		ed25519Key, ok := privKey.(ed25519.PrivateKey)
		if !ok {
			return common.Hash{}, errors.New("invalid Ed25519 key")
		}
		derivedKey = crypto.Keccak256(ed25519Key, path)
		defer zeroBytes(ed25519Key)
		
	case KeyTypeAES256:
		aesKey, ok := privKey.([]byte)
		if !ok {
			return common.Hash{}, errors.New("invalid AES key")
		}
		derivedKey = crypto.Keccak256(aesKey, path)
		defer zeroBytes(aesKey)
		
	default:
		return common.Hash{}, fmt.Errorf("unsupported key type: %d", metadata.KeyType)
	}
	
	defer zeroBytes(derivedKey)
	
	// 创建派生密钥（使用相同类型）
	childKeyID, err := ks.CreateKey(metadata.Owner, metadata.KeyType)
	if err != nil {
		return common.Hash{}, err
	}
	
	// 替换派生密钥的私钥数据
	if err := ks.savePrivateKey(childKeyID, derivedKey); err != nil {
		return common.Hash{}, err
	}
	
	return childKeyID, nil
}

// GetMetadata 获取密钥元数据
func (ks *EncryptedKeyStore) GetMetadata(keyID common.Hash) (*KeyMetadata, error) {
	metaPath := filepath.Join(ks.publicPath, keyID.Hex()+".meta")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata: %w", err)
	}
	
	var metadata KeyMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}
	
	return &metadata, nil
}

// DeleteKey 删除密钥
func (ks *EncryptedKeyStore) DeleteKey(keyID common.Hash, caller common.Address) error {
	// Load metadata to check ownership
	metadata, err := ks.GetMetadata(keyID)
	if err != nil {
		return fmt.Errorf("failed to load metadata: %w", err)
	}
	
	// Check that caller is the owner
	if metadata.Owner != caller {
		return errors.New("permission denied: only key owner can delete key")
	}
	
	// Load and zero the private key data before deleting
	keyPath := filepath.Join(ks.encryptedPath, keyID.Hex()+".key")
	data, err := os.ReadFile(keyPath)
	if err == nil {
		// Zero the key data
		zeroBytes(data)
	}
	
	// 删除私钥
	if err := os.Remove(keyPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete private key: %w", err)
	}
	
	// 删除元数据
	metaPath := filepath.Join(ks.publicPath, keyID.Hex()+".meta")
	if err := os.Remove(metaPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete metadata: %w", err)
	}
	
	return nil
}

// savePrivateKey 保存私钥到加密分区
func (ks *EncryptedKeyStore) savePrivateKey(keyID common.Hash, privKey interface{}) error {
	var data []byte
	
	switch key := privKey.(type) {
	case *ecdsa.PrivateKey:
		data = crypto.FromECDSA(key)
	case ed25519.PrivateKey:
		data = key
	case []byte:
		data = key
	default:
		return errors.New("unsupported private key type")
	}
	
	keyPath := filepath.Join(ks.encryptedPath, keyID.Hex()+".key")
	if err := os.WriteFile(keyPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write private key: %w", err)
	}
	
	return nil
}

// saveMetadata 保存元数据到公开分区
func (ks *EncryptedKeyStore) saveMetadata(metadata *KeyMetadata) error {
	data, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}
	
	metaPath := filepath.Join(ks.publicPath, metadata.KeyID.Hex()+".meta")
	if err := os.WriteFile(metaPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
	}
	
	return nil
}

// loadPrivateKey 从加密分区加载私钥
func (ks *EncryptedKeyStore) loadPrivateKey(keyID common.Hash, keyType KeyType) (interface{}, error) {
	// 从文件加载
	keyPath := filepath.Join(ks.encryptedPath, keyID.Hex()+".key")
	data, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key: %w", err)
	}
	
	// Zero file data after reading and converting
	defer zeroBytes(data)
	
	var privKey interface{}
	
	switch keyType {
	case KeyTypeECDSA:
		key, err := crypto.ToECDSA(data)
		if err != nil {
			return nil, fmt.Errorf("failed to parse ECDSA key: %w", err)
		}
		privKey = key
		
	case KeyTypeEd25519:
		if len(data) != ed25519.PrivateKeySize {
			return nil, errors.New("invalid Ed25519 key size")
		}
		// Create a copy to avoid zeroing the returned key prematurely
		keyCopy := make([]byte, len(data))
		copy(keyCopy, data)
		privKey = ed25519.PrivateKey(keyCopy)
		
	case KeyTypeAES256:
		if len(data) != 32 {
			return nil, errors.New("invalid AES key size")
		}
		// Create a copy to avoid zeroing the returned key prematurely
		keyCopy := make([]byte, len(data))
		copy(keyCopy, data)
		privKey = keyCopy
		
	default:
		return nil, fmt.Errorf("unsupported key type: %d", keyType)
	}
	
	return privKey, nil
}
