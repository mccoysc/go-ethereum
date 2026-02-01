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

// KeyType represents the cryptographic key type
type KeyType uint8

const (
	KeyTypeECDSA   KeyType = 0x01 // secp256k1
	KeyTypeEd25519 KeyType = 0x02 // Ed25519
	KeyTypeAES256  KeyType = 0x03 // AES-256
)

// KeyMetadata holds metadata about a cryptographic key
type KeyMetadata struct {
	KeyID       common.Hash    // Key identifier
	Owner       common.Address // Key owner
	KeyType     KeyType        // Key type
	CreatedAt   uint64         // Creation timestamp
	CreatedBy   common.Address // Creator address
	Permissions []Permission   // Permission list
}

// KeyStore is the interface for cryptographic key storage and operations
type KeyStore interface {
	// CreateKey creates a new cryptographic key
	CreateKey(owner common.Address, keyType KeyType) (common.Hash, error)
	
	// GetPublicKey retrieves the public key for a given key ID
	GetPublicKey(keyID common.Hash) ([]byte, error)
	
	// Sign signs data using the specified key
	Sign(keyID common.Hash, hash []byte) ([]byte, error)
	
	// ECDH performs ECDH key exchange, optionally applies KDF, and returns a new key ID
	ECDH(keyID common.Hash, peerPubKey []byte, kdfParams []byte) (common.Hash, error)
	
	// Encrypt encrypts data using the specified key
	Encrypt(keyID common.Hash, plaintext []byte) ([]byte, error)
	
	// Decrypt decrypts data using the specified key
	Decrypt(keyID common.Hash, ciphertext []byte) ([]byte, error)
	
	// DeriveKey derives a child key from a parent key
	DeriveKey(keyID common.Hash, path []byte) (common.Hash, error)
	
	// GetMetadata retrieves key metadata
	GetMetadata(keyID common.Hash) (*KeyMetadata, error)
	
	// DeleteKey deletes a key from the keystore
	DeleteKey(keyID common.Hash, caller common.Address) error
}
