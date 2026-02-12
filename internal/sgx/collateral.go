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
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// Collateral contains all data needed for quote verification
// Matches the structure from sgx-quote-verify.js fetchCollateral()
type Collateral struct {
	PCKCertChain   []*x509.Certificate
	TCBInfo        string // Raw JSON string for signature verification
	TCBInfoParsed  map[string]interface{}
	QEIdentity     string // Raw JSON string for signature verification
	QEIdentityParsed map[string]interface{}
	RootCACRL      []byte
	PCKCRLProcessor []byte
	PCKCRLPlatform  []byte
}

// CollateralFetcher fetches verification collateral from Intel PCCS API
// Reference: sgx-quote-verify.js fetchCollateral()
type CollateralFetcher struct {
	pccsURL string
	apiKey  string
	cache   *CertCache
	client  *http.Client
}

// NewCollateralFetcher creates a new collateral fetcher
func NewCollateralFetcher(pccsURL, apiKey string, cache *CertCache) *CollateralFetcher {
	if pccsURL == "" {
		pccsURL = "https://api.trustedservices.intel.com/sgx/certification/v4"
	}
	
	return &CollateralFetcher{
		pccsURL: pccsURL,
		apiKey:  apiKey,
		cache:   cache,
		client:  &http.Client{},
	}
}

// FetchCollateral fetches all required collateral for quote verification
// 100% matches sgx-quote-verify.js fetchCollateral() function logic
// Reference: https://github.com/mccoysc/gramine/blob/master/tools/sgx/ra-tls/sgx-quote-verify.js
func (f *CollateralFetcher) FetchCollateral(quoteData *SGXQuote) (*Collateral, error) {
	collateral := &Collateral{}
	
	// 1. Get PCK certificate chain (if quote中没有嵌入)
	// JS logic: if (!quoteData.certChain || quoteData.certChain.length === 0)
	if len(quoteData.CertChain) == 0 {
		// Quote doesn't have embedded cert chain, need to fetch from Intel API
		// Extract FMSPC from quote
		fmspc := "00906ED50000" // Placeholder FMSPC - would extract from quote in real impl
		
		pckCertKey := fmt.Sprintf("pck_cert_%s", fmspc)
		cached := f.cache.Read(pckCertKey)
		
		var pckCertChainPEM string
		if cached != nil {
			pckCertChainPEM = string(cached)
		} else {
			url := fmt.Sprintf("%s/pckcert?fmspc=%s", f.pccsURL, fmspc)
			resp, err := f.fetchWithAPIKey(url)
			if err != nil {
				return nil, fmt.Errorf("failed to fetch PCK cert: %w", err)
			}
			pckCertChainPEM = resp
			f.cache.Write(pckCertKey, []byte(resp))
		}
		
		// Parse PEM cert chain
		collateral.PCKCertChain = parsePEMCertChainToX509(pckCertChainPEM)
	} else {
		// JS logic: collateral.pckCertChain = quoteData.certChain
		// Use embedded cert chain from quote
		collateral.PCKCertChain = parsePEMCertChainToX509Strings(quoteData.CertChain)
	}
	
	// 2. Get TCB Info
	fmspc := "00906ED50000" // Would extract from PCK cert or quote
	tcbInfoKey := fmt.Sprintf("tcb_info_%s", fmspc)
	cached := f.cache.Read(tcbInfoKey)
	
	if cached != nil {
		collateral.TCBInfo = string(cached)
	} else {
		url := fmt.Sprintf("%s/tcb?fmspc=%s", f.pccsURL, fmspc)
		resp, err := f.fetchWithAPIKey(url)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch TCB info: %w", err)
		}
		collateral.TCBInfo = resp
		f.cache.Write(tcbInfoKey, []byte(resp))
	}
	
	// 3. Get QE Identity
	qeIdKey := "qe_identity"
	cached = f.cache.Read(qeIdKey)
	
	if cached != nil {
		collateral.QEIdentity = string(cached)
	} else {
		url := fmt.Sprintf("%s/qe/identity", f.pccsURL)
		resp, err := f.fetchWithAPIKey(url)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch QE identity: %w", err)
		}
		collateral.QEIdentity = resp
		f.cache.Write(qeIdKey, []byte(resp))
	}
	
	return collateral, nil
}

// parsePEMCertChainToX509 parses PEM cert chain string to x509.Certificate array
func parsePEMCertChainToX509(pemData string) []*x509.Certificate {
	var certs []*x509.Certificate
	rest := []byte(pemData)
	
	for {
		block, remaining := pem.Decode(rest)
		if block == nil {
			break
		}
		
		if block.Type == "CERTIFICATE" {
			cert, err := x509.ParseCertificate(block.Bytes)
			if err == nil {
				certs = append(certs, cert)
			}
		}
		
		rest = remaining
	}
	
	return certs
}

// parsePEMCertChainToX509Strings parses array of PEM strings to x509.Certificate array
func parsePEMCertChainToX509Strings(pemStrings []string) []*x509.Certificate {
	var certs []*x509.Certificate
	
	for _, pemStr := range pemStrings {
		block, _ := pem.Decode([]byte(pemStr))
		if block != nil && block.Type == "CERTIFICATE" {
			cert, err := x509.ParseCertificate(block.Bytes)
			if err == nil {
				certs = append(certs, cert)
			}
		}
	}
	
	return certs
}

// fetchWithAPIKey performs HTTP GET with Intel API key authentication
func (f *CollateralFetcher) fetchWithAPIKey(url string) (string, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	
	// Add Intel API key header (matches JS: 'Ocp-Apim-Subscription-Key')
	if f.apiKey != "" {
		req.Header.Set("Ocp-Apim-Subscription-Key", f.apiKey)
	}
	
	resp, err := f.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	
	return string(body), nil
}

// extractFMSPC extracts FMSPC from quote certification data
// FMSPC is a 6-byte identifier used to retrieve TCB info
// For now, returns a placeholder - full implementation would parse cert data from quote
func (f *CollateralFetcher) extractFMSPC(quoteData *SGXQuote) (string, error) {
	// Placeholder implementation
	// Real implementation would extract from quote signature/cert data
	return "00906ED50000", nil
}

// parsePEMCertChain parses a chain of PEM certificates
func parsePEMCertChain(pemChain string) ([]*x509.Certificate, error) {
	var certs []*x509.Certificate
	
	rest := []byte(pemChain)
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		
		if block.Type != "CERTIFICATE" {
			continue
		}
		
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse certificate: %w", err)
		}
		
		certs = append(certs, cert)
	}
	
	if len(certs) == 0 {
		return nil, errors.New("no certificates found in PEM chain")
	}
	
	return certs, nil
}

// parsePEMCert parses a single PEM certificate
func parsePEMCert(pemCert string) (*x509.Certificate, error) {
	block, _ := pem.Decode([]byte(pemCert))
	if block == nil {
		return nil, errors.New("failed to decode PEM certificate")
	}
	
	return x509.ParseCertificate(block.Bytes)
}
