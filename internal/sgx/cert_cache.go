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

package sgx

import (
	"os"
	"path/filepath"
)

// CertCache implements certificate caching to filesystem
// Matches the cacheRead/cacheWrite pattern from sgx-quote-verify.js
type CertCache struct {
	cacheDir string
}

// NewCertCache creates a new certificate cache
func NewCertCache(cacheDir string) *CertCache {
	if cacheDir == "" {
		cacheDir = "/tmp/sgx-cert-cache"
	}
	return &CertCache{
		cacheDir: cacheDir,
	}
}

// Read reads cached data by key
// Returns nil if not found (matching JS behavior: null when not cached)
func (c *CertCache) Read(key string) []byte {
	cachePath := filepath.Join(c.cacheDir, key)
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil
	}
	return data
}

// Write writes data to cache with given key
func (c *CertCache) Write(key string, data []byte) error {
	// Ensure cache directory exists
	if err := os.MkdirAll(c.cacheDir, 0755); err != nil {
		return err
	}
	
	cachePath := filepath.Join(c.cacheDir, key)
	return os.WriteFile(cachePath, data, 0644)
}

// EnsureDir ensures the cache directory exists
func (c *CertCache) EnsureDir() error {
	return os.MkdirAll(c.cacheDir, 0755)
}
