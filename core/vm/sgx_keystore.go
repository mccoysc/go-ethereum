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
	"github.com/ethereum/go-ethereum/common"
)

// KeyType 密钥类型
type KeyType uint8

const (
	KeyTypeECDSA   KeyType = 0x01 // secp256k1
	KeyTypeEd25519 KeyType = 0x02 // Ed25519
	KeyTypeAES256  KeyType = 0x03 // AES-256
)

// KeyMetadata 密钥元数据
type KeyMetadata struct {
	KeyID       common.Hash    // 密钥 ID
	Owner       common.Address // 所有者
	KeyType     KeyType        // 密钥类型
	CreatedAt   uint64         // 创建时间
	CreatedBy   common.Address // 创建者
	Permissions []Permission   // 权限列表
}

// KeyStore 密钥存储接口
type KeyStore interface {
	// CreateKey 创建新密钥
	CreateKey(owner common.Address, keyType KeyType) (common.Hash, error)
	
	// GetPublicKey 获取公钥
	GetPublicKey(keyID common.Hash) ([]byte, error)
	
	// Sign 使用密钥签名
	Sign(keyID common.Hash, hash []byte) ([]byte, error)
	
	// ECDH 执行 ECDH 密钥交换
	ECDH(keyID common.Hash, peerPubKey []byte) ([]byte, error)
	
	// Encrypt 加密数据
	Encrypt(keyID common.Hash, plaintext []byte) ([]byte, error)
	
	// Decrypt 解密数据
	Decrypt(keyID common.Hash, ciphertext []byte) ([]byte, error)
	
	// DeriveKey 派生子密钥
	DeriveKey(keyID common.Hash, path []byte) (common.Hash, error)
	
	// GetMetadata 获取密钥元数据
	GetMetadata(keyID common.Hash) (*KeyMetadata, error)
	
	// DeleteKey 删除密钥
	DeleteKey(keyID common.Hash) error
}
