//go:build !testenv
// +build !testenv

// Copyright 2024 The go-ethereum Authors
// This file is part of the go-ethereum library.

package sgx

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
)

// verifyQuoteUserData verifies that Quote userData matches the block's sealHash.
// Production version: strictly enforces userData matching.
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
		return errors.New("failed to extract userData from Quote")
	}
	
	// Verify userData length
	if len(userData) < 32 {
		return fmt.Errorf("invalid userData length: got %d, expected at least 32", len(userData))
	}
	
	// Production mode: strictly enforce userData matching
	if !bytes.Equal(userData[:32], sealHash.Bytes()) {
		log.Error("Quote userData mismatch",
			"expected", sealHash.Hex(),
			"got", common.BytesToHash(userData[:32]).Hex())
		return errors.New("Quote userData does not match seal hash - possible tampering")
	}
	
	log.Debug("✓ Quote userData verified", "sealHash", sealHash.Hex())
	return nil
}
