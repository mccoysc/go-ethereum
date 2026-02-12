//go:build testenv
// +build testenv

// Copyright 2024 The go-ethereum Authors
// This file is part of the go-ethereum library.

package sgx

import (
	"bytes"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
)

// verifyQuoteUserData verifies that Quote userData matches the block's sealHash.
// Testenv version: Executes ALL verification logic, logs results, but allows
// failures to pass (since we don't have real SGX hardware in test environment).
func (e *SGXEngine) verifyQuoteUserData(block *types.Block) error {
	header := block.Header()
	extra, err := DecodeSGXExtra(header.Extra)
	if err != nil {
		return ErrInvalidExtra
	}
	
	// Calculate seal hash
	sealHash := e.SealHash(header)
	
	// Extract userData from Quote
	userData, err := e.verifier.ExtractQuoteUserData(extra.SGXQuote)
	if err != nil {
		log.Warn("TESTENV: Failed to extract userData (would fail in production)",
			"error", err.Error())
		// In testenv, continue even if extraction fails
		return nil
	}
	
	// Verify userData length
	if len(userData) < 32 {
		log.Warn("TESTENV: Invalid userData length (would fail in production)",
			"got", len(userData),
			"expected", 32)
		// In testenv, continue even with invalid length
		return nil
	}
	
	// Execute the verification check (same as production)
	userDataMatches := bytes.Equal(userData[:32], sealHash.Bytes())
	
	if !userDataMatches {
		// Full verification FAILED - log detailed information
		log.Warn("TESTENV: Quote userData verification FAILED (allowed in testenv, would REJECT in production)",
			"blockNumber", header.Number.Uint64(),
			"expectedSealHash", sealHash.Hex(),
			"gotUserData", common.BytesToHash(userData[:32]).Hex(),
			"verificationResult", "FAILED",
			"productionBehavior", "WOULD_REJECT_BLOCK")
		
		// In testenv mode: allow block despite verification failure
		// This lets us test all other logic without requiring real SGX
		return nil
	}
	
	// Verification PASSED
	log.Info("TESTENV: Quote userData verification PASSED",
		"blockNumber", header.Number.Uint64(),
		"sealHash", sealHash.Hex(),
		"verificationResult", "PASSED")
	
	return nil
}
