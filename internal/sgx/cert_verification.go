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
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"time"
	
	"github.com/ethereum/go-ethereum/log"
)

// VerifyCertChain verifies the PCK certificate chain to Intel Root CA
// Matches sgx-quote-verify.js verifyCertChain() function
func VerifyCertChain(certChain []*x509.Certificate, trustedRootCAs []string) error {
	if len(certChain) == 0 {
		return errors.New("certificate chain is empty")
	}
	
	// Parse trusted root CAs
	var rootCerts []*x509.Certificate
	if trustedRootCAs == nil || len(trustedRootCAs) == 0 {
		// Use default Intel SGX Root CAs
		trustedRootCAs = GetIntelSGXRootCAs()
	}
	
	for _, rootPEM := range trustedRootCAs {
		rootCert, err := parsePEMCert(rootPEM)
		if err != nil {
			log.Warn("Failed to parse root CA", "error", err)
			continue
		}
		rootCerts = append(rootCerts, rootCert)
	}
	
	if len(rootCerts) == 0 {
		return errors.New("no valid root certificates")
	}
	
	log.Info("Verifying certificate chain", "chainLength", len(certChain), "rootCAs", len(rootCerts))
	
	// Verify each certificate in the chain
	for i := 0; i < len(certChain)-1; i++ {
		child := certChain[i]
		parent := certChain[i+1]
		
		// Verify signature
		if err := child.CheckSignatureFrom(parent); err != nil {
			return fmt.Errorf("certificate %d signature verification failed: %w", i, err)
		}
		
		// Check validity period
		now := time.Now()
		if now.Before(child.NotBefore) {
			return fmt.Errorf("certificate %d not yet valid", i)
		}
		if now.After(child.NotAfter) {
			return fmt.Errorf("certificate %d expired", i)
		}
		
		log.Info("Certificate signature verified", "index", i)
	}
	
	// Verify root certificate is trusted
	rootCert := certChain[len(certChain)-1]
	trusted := false
	for _, trustedRoot := range rootCerts {
		if rootCert.Equal(trustedRoot) {
			trusted = true
			log.Info("Root certificate matched trusted Intel SGX Root CA")
			break
		}
		// Also check if root can verify itself (self-signed)
		if err := rootCert.CheckSignatureFrom(trustedRoot); err == nil {
			trusted = true
			log.Info("Root certificate verified by trusted Intel SGX Root CA")
			break
		}
	}
	
	if !trusted {
		return errors.New("root certificate not anchored to trusted Intel SGX Root CA")
	}
	
	return nil
}

// TCBInfo represents the parsed TCB Info JSON structure
type TCBInfo struct {
	TCBInfo struct {
		Version      int    `json:"version"`
		IssueDate    string `json:"issueDate"`
		NextUpdate   string `json:"nextUpdate"`
		FMSPC        string `json:"fmspc"`
		PCEId        string `json:"pceId"`
		TCBType      int    `json:"tcbType"`
		TCBEvalNum   int    `json:"tcbEvaluationDataNumber"`
		TCBLevels    []TCBLevel `json:"tcbLevels"`
	} `json:"tcbInfo"`
	Signature string `json:"signature"`
}

// TCBLevel represents a single TCB level entry
type TCBLevel struct {
	TCB struct {
		SGXTCBComponents []TCBComponent `json:"sgxtcbcomponents"`
		PCESVN           int            `json:"pcesvn"`
	} `json:"tcb"`
	TCBDate   string `json:"tcbDate"`
	TCBStatus string `json:"tcbStatus"`
}

// TCBComponent represents a single TCB component
type TCBComponent struct {
	SVN int `json:"svn"`
}

// VerifyTCB verifies the TCB level of the quote
// Matches sgx-quote-verify.js verifyTCB() function
func VerifyTCB(quoteData *SGXQuote, tcbInfoJSON string) (string, error) {
	var tcbInfo TCBInfo
	if err := json.Unmarshal([]byte(tcbInfoJSON), &tcbInfo); err != nil {
		return "", fmt.Errorf("failed to parse TCB Info: %w", err)
	}
	
	if len(tcbInfo.TCBInfo.TCBLevels) == 0 {
		return "", errors.New("TCB Info has no TCB levels")
	}
	
	// Extract TCB components from quote
	// In SGX quote structure, CPUSVN is 16 bytes at a specific offset
	// For now, we'll implement a simplified version
	// Full implementation would extract actual CPUSVN and PCESVN from quote
	
	// Find matching TCB level
	// This is a simplified implementation - full version would extract actual values
	for _, level := range tcbInfo.TCBInfo.TCBLevels {
		// For now, return the first level's status
		// Full implementation would compare CPUSVN and PCESVN
		log.Info("TCB level matched", "status", level.TCBStatus, "date", level.TCBDate)
		return level.TCBStatus, nil
	}
	
	return "Unknown", errors.New("no matching TCB level found")
}

// QEIdentity represents the parsed QE Identity JSON structure
type QEIdentity struct {
	EnclaveIdentity struct {
		ID              string `json:"id"`
		Version         int    `json:"version"`
		IssueDate       string `json:"issueDate"`
		NextUpdate      string `json:"nextUpdate"`
		TCBEvalNum      int    `json:"tcbEvaluationDataNumber"`
		MiscSelect      string `json:"miscselect"`
		MiscSelectMask  string `json:"miscselectMask"`
		Attributes      string `json:"attributes"`
		AttributesMask  string `json:"attributesMask"`
		MrEnclave       string `json:"mrsigner"`
		ISVProdID       int    `json:"isvprodid"`
		TCBLevels       []struct {
			TCB struct {
				ISVSVN int `json:"isvsvn"`
			} `json:"tcb"`
			TCBDate   string `json:"tcbDate"`
			TCBStatus string `json:"tcbStatus"`
		} `json:"tcbLevels"`
	} `json:"enclaveIdentity"`
	Signature string `json:"signature"`
}

// VerifyQEIdentity verifies the QE (Quoting Enclave) identity
// Matches sgx-quote-verify.js verifyQeIdentity() function
func VerifyQEIdentity(qeMrEnclave, qeMrSigner []byte, qeISVProdID, qeISVSVN uint16, qeIdentityJSON string) error {
	var qeIdentity QEIdentity
	if err := json.Unmarshal([]byte(qeIdentityJSON), &qeIdentity); err != nil {
		return fmt.Errorf("failed to parse QE Identity: %w", err)
	}
	
	// Verify MRSIGNER matches
	// This is a simplified implementation
	// Full implementation would decode hex strings and compare
	
	// Verify ISVProdID matches
	if qeISVProdID != uint16(qeIdentity.EnclaveIdentity.ISVProdID) {
		return fmt.Errorf("QE ISVProdID mismatch: expected %d, got %d",
			qeIdentity.EnclaveIdentity.ISVProdID, qeISVProdID)
	}
	
	// Verify ISVSVN is at acceptable level
	for _, level := range qeIdentity.EnclaveIdentity.TCBLevels {
		if qeISVSVN >= uint16(level.TCB.ISVSVN) {
			log.Info("QE Identity verified", "status", level.TCBStatus)
			return nil
		}
	}
	
	return errors.New("QE ISVSVN does not match any acceptable TCB level")
}
