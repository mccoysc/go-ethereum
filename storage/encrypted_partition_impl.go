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

import (
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// EncryptedPartitionImpl implements EncryptedPartition using Gramine's transparent encryption
// Gramine automatically encrypts/decrypts files in the configured encrypted filesystem
type EncryptedPartitionImpl struct {
	mu       sync.RWMutex
	basePath string
}

// NewEncryptedPartition creates a new encrypted partition manager
// The basePath should point to a directory configured in Gramine manifest as encrypted
func NewEncryptedPartition(basePath string) (*EncryptedPartitionImpl, error) {
	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("encrypted partition path does not exist: %s", basePath)
	}

	return &EncryptedPartitionImpl{
		basePath: basePath,
	}, nil
}

// WriteSecret writes secret data to the encrypted partition
// Gramine transparently encrypts the data when it's written to disk
func (ep *EncryptedPartitionImpl) WriteSecret(id string, data []byte) error {
	ep.mu.Lock()
	defer ep.mu.Unlock()

	filePath := filepath.Join(ep.basePath, id)

	// Standard file write - Gramine handles encryption transparently
	file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	if _, err := file.Write(data); err != nil {
		return fmt.Errorf("failed to write data: %w", err)
	}

	return nil
}

// ReadSecret reads secret data from the encrypted partition
// Gramine transparently decrypts the data when it's read from disk
func (ep *EncryptedPartitionImpl) ReadSecret(id string) ([]byte, error) {
	ep.mu.RLock()
	defer ep.mu.RUnlock()

	filePath := filepath.Join(ep.basePath, id)

	// Standard file read - Gramine handles decryption transparently
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read secret: %w", err)
	}

	return data, nil
}

// DeleteSecret securely deletes secret data
func (ep *EncryptedPartitionImpl) DeleteSecret(id string) error {
	ep.mu.Lock()
	defer ep.mu.Unlock()

	filePath := filepath.Join(ep.basePath, id)

	// Perform secure deletion
	if err := ep.SecureDelete(filePath); err != nil {
		return err
	}

	return nil
}

// SecureDelete securely deletes a file by overwriting it with random data first
func (ep *EncryptedPartitionImpl) SecureDelete(filePath string) error {
	// Get file size
	info, err := os.Stat(filePath)
	if err != nil {
		return err
	}

	// Overwrite with random data
	file, err := os.OpenFile(filePath, os.O_WRONLY, 0600)
	if err != nil {
		return err
	}

	randomData := make([]byte, info.Size())
	if _, err := io.ReadFull(rand.Reader, randomData); err != nil {
		file.Close()
		return err
	}

	if _, err := file.Write(randomData); err != nil {
		file.Close()
		return err
	}
	file.Close()

	// Delete the file
	return os.Remove(filePath)
}

// ListSecrets lists all secret IDs in the partition
func (ep *EncryptedPartitionImpl) ListSecrets() ([]string, error) {
	ep.mu.RLock()
	defer ep.mu.RUnlock()

	entries, err := os.ReadDir(ep.basePath)
	if err != nil {
		return nil, err
	}

	ids := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			ids = append(ids, entry.Name())
		}
	}

	return ids, nil
}
