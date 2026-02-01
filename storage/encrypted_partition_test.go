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
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewEncryptedPartition(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "encrypted-partition-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Test successful creation
	partition, err := NewEncryptedPartition(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create encrypted partition: %v", err)
	}

	if partition == nil {
		t.Fatal("Partition is nil")
	}

	if partition.basePath != tmpDir {
		t.Errorf("Expected basePath %s, got %s", tmpDir, partition.basePath)
	}
}

func TestNewEncryptedPartition_NonExistentPath(t *testing.T) {
	// Test with non-existent path
	_, err := NewEncryptedPartition("/non/existent/path")
	if err == nil {
		t.Fatal("Expected error for non-existent path")
	}
}

func TestWriteAndReadSecret(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "encrypted-partition-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	partition, err := NewEncryptedPartition(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create partition: %v", err)
	}

	// Test data
	secretID := "test-secret"
	secretData := []byte("this is a test secret")

	// Write secret
	err = partition.WriteSecret(secretID, secretData)
	if err != nil {
		t.Fatalf("Failed to write secret: %v", err)
	}

	// Read secret
	readData, err := partition.ReadSecret(secretID)
	if err != nil {
		t.Fatalf("Failed to read secret: %v", err)
	}

	// Verify data
	if !bytes.Equal(readData, secretData) {
		t.Errorf("Read data doesn't match written data. Got %v, want %v", readData, secretData)
	}
}

func TestDeleteSecret(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "encrypted-partition-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	partition, err := NewEncryptedPartition(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create partition: %v", err)
	}

	secretID := "test-secret"
	secretData := []byte("secret data")

	// Write secret
	err = partition.WriteSecret(secretID, secretData)
	if err != nil {
		t.Fatalf("Failed to write secret: %v", err)
	}

	// Delete secret
	err = partition.DeleteSecret(secretID)
	if err != nil {
		t.Fatalf("Failed to delete secret: %v", err)
	}

	// Verify it's deleted
	_, err = partition.ReadSecret(secretID)
	if err == nil {
		t.Fatal("Expected error reading deleted secret")
	}
}

func TestListSecrets(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "encrypted-partition-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	partition, err := NewEncryptedPartition(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create partition: %v", err)
	}

	// Write multiple secrets
	secrets := map[string][]byte{
		"secret1": []byte("data1"),
		"secret2": []byte("data2"),
		"secret3": []byte("data3"),
	}

	for id, data := range secrets {
		err = partition.WriteSecret(id, data)
		if err != nil {
			t.Fatalf("Failed to write secret %s: %v", id, err)
		}
	}

	// List secrets
	ids, err := partition.ListSecrets()
	if err != nil {
		t.Fatalf("Failed to list secrets: %v", err)
	}

	if len(ids) != len(secrets) {
		t.Errorf("Expected %d secrets, got %d", len(secrets), len(ids))
	}

	// Verify all secrets are listed
	idMap := make(map[string]bool)
	for _, id := range ids {
		idMap[id] = true
	}

	for id := range secrets {
		if !idMap[id] {
			t.Errorf("Secret %s not found in list", id)
		}
	}
}

func TestSecureDelete(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "encrypted-partition-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	partition, err := NewEncryptedPartition(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create partition: %v", err)
	}

	// Create a test file
	testFile := filepath.Join(tmpDir, "test-file")
	testData := []byte("sensitive data to be securely deleted")

	err = os.WriteFile(testFile, testData, 0600)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Secure delete
	err = partition.SecureDelete(testFile)
	if err != nil {
		t.Fatalf("Failed to secure delete: %v", err)
	}

	// Verify file is deleted
	_, err = os.Stat(testFile)
	if !os.IsNotExist(err) {
		t.Error("File should not exist after secure delete")
	}
}

func TestConcurrentWriteAndRead(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "encrypted-partition-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	partition, err := NewEncryptedPartition(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create partition: %v", err)
	}

	// Test concurrent writes and reads
	done := make(chan bool)
	errChan := make(chan error, 20)

	// Start multiple writers
	for i := 0; i < 10; i++ {
		go func(id int) {
			secretID := fmt.Sprintf("secret-%d", id)
			secretData := []byte{byte(id)}
			err := partition.WriteSecret(secretID, secretData)
			if err != nil {
				errChan <- err
			}
			done <- true
		}(i)
	}

	// Start multiple readers
	for i := 0; i < 10; i++ {
		go func(id int) {
			secretID := fmt.Sprintf("secret-%d", id)
			// Allow some time for write to complete
			time.Sleep(10 * time.Millisecond)
			_, err := partition.ReadSecret(secretID)
			// It's ok if read fails (race condition), just don't panic
			if err != nil {
				// Expected in concurrent scenario
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 20; i++ {
		select {
		case <-done:
		case err := <-errChan:
			t.Errorf("Concurrent operation error: %v", err)
		}
	}
}
