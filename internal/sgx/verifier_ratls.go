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

// +build !cgo

package sgx

import (
	"crypto/x509"
	"fmt"
	"sync"
)

// GramineRATLSVerifier stub for non-CGO builds.
type GramineRATLSVerifier struct {
	mu               sync.RWMutex
	allowedMREnclave map[string]bool
	allowedMRSigner  map[string]bool
	allowOutdatedTCB bool
}

func NewGramineRATLSVerifier(allowOutdatedTCB bool) *GramineRATLSVerifier {
	return &GramineRATLSVerifier{
		allowedMREnclave: make(map[string]bool),
		allowedMRSigner:  make(map[string]bool),
		allowOutdatedTCB: allowOutdatedTCB,
	}
}

func (v *GramineRATLSVerifier) VerifyQuote(quote []byte) error {
	// Parse the quote to check basic structure
	parsedQuote, err := ParseQuote(quote)
	if err != nil {
		return fmt.Errorf("failed to parse quote: %w", err)
	}
	
	// Check MRENCLAVE whitelist
	if !v.IsAllowedMREnclave(parsedQuote.MRENCLAVE[:]) {
		return fmt.Errorf("MRENCLAVE not in allowed list: %x", parsedQuote.MRENCLAVE)
	}
	
	// Note: Full verification requires CGO and Gramine libraries
	return nil
}

func (v *GramineRATLSVerifier) VerifyCertificate(cert *x509.Certificate) error {
	return fmt.Errorf("Gramine RA-TLS certificate verification requires CGO support")
}

func (v *GramineRATLSVerifier) IsAllowedMREnclave(mrenclave []byte) bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	
	if len(v.allowedMREnclave) == 0 {
		return true
	}
	
	return v.allowedMREnclave[string(mrenclave)]
}

func (v *GramineRATLSVerifier) AddAllowedMREnclave(mrenclave []byte) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.allowedMREnclave[string(mrenclave)] = true
}

func (v *GramineRATLSVerifier) RemoveAllowedMREnclave(mrenclave []byte) {
	v.mu.Lock()
	defer v.mu.Unlock()
	delete(v.allowedMREnclave, string(mrenclave))
}

