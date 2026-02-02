package sgx

import (
	"errors"
	
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

var (
	ErrInvalidMREnclave = errors.New("invalid MRENCLAVE")
)

// DefaultAttestor provides a default implementation of the Attestor interface
// Mock implementation for now - real SGX integration would use gramine
type DefaultAttestor struct {
}

func NewDefaultAttestor() *DefaultAttestor {
	return &DefaultAttestor{}
}

func (a *DefaultAttestor) GenerateQuote(data []byte) ([]byte, error) {
	// Mock implementation - return dummy quote
	return data, nil
}

func (a *DefaultAttestor) GetMREnclave() ([]byte, error) {
	// Mock implementation - return dummy MRENCLAVE
	return make([]byte, 32), nil
}

func (a *DefaultAttestor) GetMRSigner() ([]byte, error) {
	// Mock implementation - return dummy MRSIGNER
	return make([]byte, 32), nil
}

func (a *DefaultAttestor) SignBlock(block *types.Block) ([]byte, error) {
	// Use SGX to sign the block hash
	hash := block.Hash()
	return hash.Bytes(), nil
}

func (a *DefaultAttestor) SignInEnclave(data []byte) ([]byte, error) {
	// Mock implementation - return dummy signature (65 bytes for ECDSA)
	sig := make([]byte, 65)
	copy(sig, data)
	return sig, nil
}

func (a *DefaultAttestor) GetProducerID() ([]byte, error) {
	// Return producer ID from MRENCLAVE (20 bytes for Ethereum address)
	id := make([]byte, 20)
	mrenclave, err := a.GetMREnclave()
	if err == nil && len(mrenclave) >= 20 {
		copy(id, mrenclave[:20])
	}
	return id, nil
}

// DefaultVerifier provides a default implementation of the Verifier interface
type DefaultVerifier struct {
}

func NewDefaultVerifier() *DefaultVerifier {
	return &DefaultVerifier{}
}

func (v *DefaultVerifier) VerifyQuote(quote []byte) error {
	// Mock implementation - always succeed
	return nil
}

func (v *DefaultVerifier) VerifyMREnclave(mrenclave []byte, expected []byte) error {
	// Compare MREnclave values
	if len(mrenclave) != len(expected) {
		return ErrInvalidMREnclave
	}
	for i := range mrenclave {
		if mrenclave[i] != expected[i] {
			return ErrInvalidMREnclave
		}
	}
	return nil
}

func (v *DefaultVerifier) VerifyBlockSignature(block *types.Block, signature []byte, signer common.Address) error {
	// Mock implementation - verify signature
	return nil
}

func (v *DefaultVerifier) VerifySignature(data, signature, producerID []byte) error {
	// Mock implementation - always succeed
	return nil
}

func (v *DefaultVerifier) ExtractProducerID(quote []byte) ([]byte, error) {
	// Extract producer ID from quote (first 20 bytes for Ethereum address)
	if len(quote) >= 20 {
		return quote[:20], nil
	}
	return quote, nil
}
