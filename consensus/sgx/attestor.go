package sgx

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/internal/sgx"
)

// DefaultAttestor provides a default implementation of the Attestor interface
// It integrates with the internal SGX attestation module (Module 01)
type DefaultAttestor struct {
	gramine *sgx.GramineAttestation
}

func NewDefaultAttestor() *DefaultAttestor {
	return &DefaultAttestor{
		gramine: sgx.NewGramineAttestation(),
	}
}

func (a *DefaultAttestor) GenerateQuote(data []byte) ([]byte, error) {
	return a.gramine.GenerateQuote(data)
}

func (a *DefaultAttestor) GetMREnclave() ([]byte, error) {
	return a.gramine.GetMREnclave()
}

func (a *DefaultAttestor) GetMRSigner() ([]byte, error) {
	return a.gramine.GetMRSigner()
}

func (a *DefaultAttestor) SignBlock(block *types.Block) ([]byte, error) {
	// Use SGX to sign the block hash
	hash := block.Hash()
	return a.gramine.GenerateQuote(hash.Bytes())
}

// DefaultVerifier provides a default implementation of the Verifier interface
// It integrates with the internal SGX verification module
type DefaultVerifier struct {
	gramine *sgx.GramineAttestation
}

func NewDefaultVerifier() *DefaultVerifier {
	return &DefaultVerifier{
		gramine: sgx.NewGramineAttestation(),
	}
}

func (v *DefaultVerifier) VerifyQuote(quote []byte) error {
	return v.gramine.VerifyQuote(quote)
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
	// Verify the SGX quote/signature
	return v.gramine.VerifyQuote(signature)
}
