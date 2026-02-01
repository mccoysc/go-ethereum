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

package storage

// EncryptedPartition interface for managing encrypted storage
// Gramine provides transparent encryption/decryption of files
type EncryptedPartition interface {
	// WriteSecret writes secret data to encrypted partition
	WriteSecret(id string, data []byte) error

	// ReadSecret reads secret data from encrypted partition
	ReadSecret(id string) ([]byte, error)

	// DeleteSecret securely deletes secret data
	DeleteSecret(id string) error

	// ListSecrets lists all secret IDs in the partition
	ListSecrets() ([]string, error)

	// SecureDelete performs a secure deletion of a file
	SecureDelete(filePath string) error
}
