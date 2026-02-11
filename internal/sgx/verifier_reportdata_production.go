//go:build !testenv
// +build !testenv

// Copyright 2024 The go-ethereum Authors
// This file is part of the go-ethereum library.

package sgx

import (
	"errors"
)

// verifyReportDataMatch verifies that the quote's report data matches expected data.
// Production version: strict comparison - must match exactly.
func verifyReportDataMatch(reportData, expected []byte, compareLen int) error {
	reportDataToCompare := reportData[:compareLen]
	expectedToCompare := expected[:compareLen]
	
	if !ConstantTimeCompare(reportDataToCompare, expectedToCompare) {
		return errors.New("report data does not match expected value")
	}
	
	return nil
}
