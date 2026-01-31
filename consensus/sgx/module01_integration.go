package sgx

import (
	"crypto/ecdsa"

	internalsgx "github.com/ethereum/go-ethereum/internal/sgx"
)

// NewWithModule01 creates an SGX consensus engine using the actual Module 01 implementation
// This is the recommended way to create a production SGX engine
func NewWithModule01(config *Config, privateKey *ecdsa.PrivateKey) (*SGXEngine, error) {
	// Create Module 01 attestor
	m01Attestor, err := internalsgx.NewGramineAttestor()
	if err != nil {
		return nil, err
	}

	// Create Module 01 verifier
	m01Verifier := internalsgx.NewDCAPVerifier(false) // Don't allow outdated TCB

	// Wrap with adapters to add consensus-specific methods
	attestor := NewAttestorAdapter(m01Attestor, privateKey)
	verifier := NewVerifierAdapter(m01Verifier)

	// Create the engine
	return New(config, attestor, verifier), nil
}

// NewWithModule01AndMRENCLAVEWhitelist creates an SGX consensus engine with MRENCLAVE whitelist
func NewWithModule01AndMRENCLAVEWhitelist(config *Config, privateKey *ecdsa.PrivateKey, allowedMREnclaves [][]byte) (*SGXEngine, error) {
	// Create Module 01 attestor
	m01Attestor, err := internalsgx.NewGramineAttestor()
	if err != nil {
		return nil, err
	}

	// Create Module 01 verifier
	m01Verifier := internalsgx.NewDCAPVerifier(false)

	// Add MRENCLAVE whitelist
	for _, mrenclave := range allowedMREnclaves {
		m01Verifier.AddAllowedMREnclave(mrenclave)
	}

	// Wrap with adapters
	attestor := NewAttestorAdapter(m01Attestor, privateKey)
	verifier := NewVerifierAdapter(m01Verifier)

	// Create the engine
	return New(config, attestor, verifier), nil
}
