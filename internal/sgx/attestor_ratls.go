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

//go:build !cgo
// +build !cgo

package sgx

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
)

// GramineRATLSAttestor stub for non-CGO builds.
// This allows the code to compile without CGO, but will return errors if used.
type GramineRATLSAttestor struct{}

// NewGramineRATLSAttestor returns an error in non-CGO builds.
func NewGramineRATLSAttestor() (*GramineRATLSAttestor, error) {
	return nil, fmt.Errorf("Gramine RA-TLS requires CGO support")
}

func (a *GramineRATLSAttestor) GenerateQuote(reportData []byte) ([]byte, error) {
	return nil, fmt.Errorf("Gramine RA-TLS requires CGO support")
}

func (a *GramineRATLSAttestor) GenerateCertificate() (*tls.Certificate, error) {
	return nil, fmt.Errorf("Gramine RA-TLS requires CGO support")
}

func (a *GramineRATLSAttestor) GetMREnclave() []byte {
	return nil
}

func (a *GramineRATLSAttestor) GetMRSigner() []byte {
	return nil
}

// GramineRATLSVerifier stub for non-CGO builds.
type GramineRATLSVerifier struct{}

func NewGramineRATLSVerifier(allowOutdatedTCB bool) *GramineRATLSVerifier {
	return &GramineRATLSVerifier{}
}

func (v *GramineRATLSVerifier) VerifyQuote(quote []byte) error {
	return fmt.Errorf("Gramine RA-TLS requires CGO support")
}

func (v *GramineRATLSVerifier) VerifyCertificate(cert *x509.Certificate) error {
	return fmt.Errorf("Gramine RA-TLS requires CGO support")
}

func (v *GramineRATLSVerifier) IsAllowedMREnclave(mrenclave []byte) bool {
	return false
}

func (v *GramineRATLSVerifier) AddAllowedMREnclave(mrenclave []byte) {}

func (v *GramineRATLSVerifier) RemoveAllowedMREnclave(mrenclave []byte) {}
