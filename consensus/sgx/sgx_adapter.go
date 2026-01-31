package sgx

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	internalsgx "github.com/ethereum/go-ethereum/internal/sgx"
)

// AttestorAdapter wraps internal/sgx.Attestor and adds consensus-specific methods
type AttestorAdapter struct {
	internalsgx.Attestor
	privateKey *ecdsa.PrivateKey // 用于签名的私钥
}

// NewAttestorAdapter creates a new attestor adapter wrapping Module 01's implementation
func NewAttestorAdapter(attestor internalsgx.Attestor, privateKey *ecdsa.PrivateKey) *AttestorAdapter {
	return &AttestorAdapter{
		Attestor:   attestor,
		privateKey: privateKey,
	}
}

// SignInEnclave signs data using the enclave's private key
// In a real SGX implementation, this would use the key sealed in the enclave
// For now, we use the provided private key
func (a *AttestorAdapter) SignInEnclave(data []byte) ([]byte, error) {
	if a.privateKey == nil {
		return nil, fmt.Errorf("no private key available for signing")
	}

	// Sign the data using ECDSA
	hash := crypto.Keccak256Hash(data)
	signature, err := crypto.Sign(hash.Bytes(), a.privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign data: %w", err)
	}

	return signature, nil
}

// GetProducerID returns the producer ID (Ethereum address derived from public key)
func (a *AttestorAdapter) GetProducerID() ([]byte, error) {
	if a.privateKey == nil {
		return nil, fmt.Errorf("no private key available")
	}

	// Derive Ethereum address from public key
	address := crypto.PubkeyToAddress(a.privateKey.PublicKey)
	return address.Bytes(), nil
}

// VerifierAdapter wraps internal/sgx.Verifier and adds consensus-specific methods
type VerifierAdapter struct {
	internalsgx.Verifier
}

// NewVerifierAdapter creates a new verifier adapter wrapping Module 01's implementation
func NewVerifierAdapter(verifier internalsgx.Verifier) *VerifierAdapter {
	return &VerifierAdapter{
		Verifier: verifier,
	}
}

// VerifySignature verifies an ECDSA signature using the producer ID (Ethereum address)
func (v *VerifierAdapter) VerifySignature(data, signature, producerID []byte) error {
	if len(signature) != 65 {
		return fmt.Errorf("invalid signature length: expected 65, got %d", len(signature))
	}

	if len(producerID) != 20 {
		return fmt.Errorf("invalid producer ID length: expected 20, got %d", len(producerID))
	}

	// Hash the data
	hash := crypto.Keccak256Hash(data)

	// Recover the public key from the signature
	pubKey, err := crypto.SigToPub(hash.Bytes(), signature)
	if err != nil {
		return fmt.Errorf("failed to recover public key: %w", err)
	}

	// Derive the address from the recovered public key
	recoveredAddress := crypto.PubkeyToAddress(*pubKey)

	// Compare with the expected producer ID
	expectedAddress := common.BytesToAddress(producerID)
	if recoveredAddress != expectedAddress {
		return fmt.Errorf("signature verification failed: expected %s, got %s",
			expectedAddress.Hex(), recoveredAddress.Hex())
	}

	return nil
}

// ExtractProducerID extracts the producer ID from an SGX quote
// The producer ID is derived from the public key embedded in the quote's report data
func (v *VerifierAdapter) ExtractProducerID(quote []byte) ([]byte, error) {
	// Parse the quote to extract report data
	reportData, err := internalsgx.ExtractReportData(quote)
	if err != nil {
		return nil, fmt.Errorf("failed to extract report data: %w", err)
	}

	// The report data should contain the public key or its hash
	// For now, we'll use a hash of the report data as the producer ID
	// In a real implementation, you would:
	// 1. Parse the public key from report data
	// 2. Derive the Ethereum address from it
	
	// Simple approach: use first 20 bytes of the hash of report data as address
	hash := sha256.Sum256(reportData)
	producerID := hash[:20]

	return producerID, nil
}
