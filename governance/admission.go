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

package governance

import (
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/internal/sgx"
)

// SGXAdmissionController implements AdmissionController with SGX verification
type SGXAdmissionController struct {
	mu                  sync.RWMutex
	whitelist           WhitelistManager
	verifier            SGXVerifier
	status              map[common.Hash]*AdmissionStatus
	hardwareToValidator map[string]common.Address
	validatorToHardware map[common.Address]string
}

// NewSGXAdmissionController creates a new SGX admission controller
func NewSGXAdmissionController(whitelist WhitelistManager, verifier SGXVerifier) *SGXAdmissionController {
	return &SGXAdmissionController{
		whitelist:           whitelist,
		verifier:            verifier,
		status:              make(map[common.Hash]*AdmissionStatus),
		hardwareToValidator: make(map[string]common.Address),
		validatorToHardware: make(map[common.Address]string),
	}
}

// CheckAdmission checks if a node is allowed to join
func (ac *SGXAdmissionController) CheckAdmission(nodeID common.Hash, mrenclave [32]byte, quote []byte) (bool, error) {
	ac.mu.Lock()
	defer ac.mu.Unlock()

	// 1. Verify SGX quote
	if err := ac.verifier.VerifyQuote(quote); err != nil {
		status := &AdmissionStatus{
			NodeID:       nodeID,
			MRENCLAVE:    mrenclave,
			Allowed:      false,
			Reason:       "Quote verification failed: " + err.Error(),
			LastVerified: uint64(time.Now().Unix()),
		}
		ac.status[nodeID] = status
		return false, ErrQuoteVerificationFailed
	}

	// 2. Extract MRENCLAVE from quote
	quoteMREnclave, err := ac.verifier.ExtractMREnclave(quote)
	if err != nil {
		status := &AdmissionStatus{
			NodeID:       nodeID,
			MRENCLAVE:    mrenclave,
			Allowed:      false,
			Reason:       "Failed to extract MRENCLAVE: " + err.Error(),
			LastVerified: uint64(time.Now().Unix()),
		}
		ac.status[nodeID] = status
		return false, err
	}

	// 3. Verify MRENCLAVE matches claimed value
	if quoteMREnclave != mrenclave {
		status := &AdmissionStatus{
			NodeID:       nodeID,
			MRENCLAVE:    mrenclave,
			Allowed:      false,
			Reason:       "MRENCLAVE mismatch",
			LastVerified: uint64(time.Now().Unix()),
		}
		ac.status[nodeID] = status
		return false, ErrInvalidMREnclave
	}

	// 4. Check whitelist
	if !ac.whitelist.IsAllowed(mrenclave) {
		status := &AdmissionStatus{
			NodeID:       nodeID,
			MRENCLAVE:    mrenclave,
			Allowed:      false,
			Reason:       "MRENCLAVE not in whitelist",
			LastVerified: uint64(time.Now().Unix()),
		}
		ac.status[nodeID] = status
		return false, ErrMREnclaveNotAllowed
	}

	// 5. Extract hardware ID
	hardwareID, err := ac.verifier.ExtractHardwareID(quote)
	if err != nil {
		status := &AdmissionStatus{
			NodeID:       nodeID,
			MRENCLAVE:    mrenclave,
			Allowed:      false,
			Reason:       "Failed to extract hardware ID: " + err.Error(),
			LastVerified: uint64(time.Now().Unix()),
		}
		ac.status[nodeID] = status
		return false, err
	}

	// 6. Check hardware uniqueness (one validator per SGX CPU)
	if existingValidator, exists := ac.hardwareToValidator[hardwareID]; exists {
		// Check if it's the same validator reconnecting
		existingHW, _ := ac.validatorToHardware[existingValidator]
		if existingHW != hardwareID {
			status := &AdmissionStatus{
				NodeID:       nodeID,
				MRENCLAVE:    mrenclave,
				Allowed:      false,
				Reason:       "Hardware already registered to another validator",
				LastVerified: uint64(time.Now().Unix()),
			}
			ac.status[nodeID] = status
			return false, ErrHardwareAlreadyRegistered
		}
	}

	// Admission granted
	status := &AdmissionStatus{
		NodeID:       nodeID,
		MRENCLAVE:    mrenclave,
		Allowed:      true,
		Reason:       "Admission granted",
		ConnectedAt:  uint64(time.Now().Unix()),
		LastVerified: uint64(time.Now().Unix()),
	}
	ac.status[nodeID] = status

	return true, nil
}

// GetAdmissionStatus returns the admission status of a node
func (ac *SGXAdmissionController) GetAdmissionStatus(nodeID common.Hash) (*AdmissionStatus, error) {
	ac.mu.RLock()
	defer ac.mu.RUnlock()

	status, exists := ac.status[nodeID]
	if !exists {
		return nil, ErrNodeNotFound
	}

	statusCopy := *status
	return &statusCopy, nil
}

// RecordConnection records a node connection
func (ac *SGXAdmissionController) RecordConnection(nodeID common.Hash, mrenclave [32]byte) error {
	ac.mu.Lock()
	defer ac.mu.Unlock()

	status, exists := ac.status[nodeID]
	if !exists {
		status = &AdmissionStatus{
			NodeID:    nodeID,
			MRENCLAVE: mrenclave,
		}
		ac.status[nodeID] = status
	}

	status.ConnectedAt = uint64(time.Now().Unix())
	status.Allowed = true

	return nil
}

// RecordDisconnection records a node disconnection
func (ac *SGXAdmissionController) RecordDisconnection(nodeID common.Hash) error {
	ac.mu.Lock()
	defer ac.mu.Unlock()

	status, exists := ac.status[nodeID]
	if !exists {
		return ErrNodeNotFound
	}

	status.ConnectedAt = 0

	return nil
}

// GetHardwareBinding returns the hardware ID binding for a validator
func (ac *SGXAdmissionController) GetHardwareBinding(validatorAddr common.Address) (string, bool) {
	ac.mu.RLock()
	defer ac.mu.RUnlock()

	hardwareID, exists := ac.validatorToHardware[validatorAddr]
	return hardwareID, exists
}

// GetValidatorByHardware returns the validator address for a hardware ID
func (ac *SGXAdmissionController) GetValidatorByHardware(hardwareID string) (common.Address, bool) {
	ac.mu.RLock()
	defer ac.mu.RUnlock()

	validator, exists := ac.hardwareToValidator[hardwareID]
	return validator, exists
}

// UnregisterValidator removes the hardware binding for a validator
func (ac *SGXAdmissionController) UnregisterValidator(validatorAddr common.Address) error {
	ac.mu.Lock()
	defer ac.mu.Unlock()

	hardwareID, exists := ac.validatorToHardware[validatorAddr]
	if !exists {
		return ErrValidatorNotFound
	}

	delete(ac.validatorToHardware, validatorAddr)
	delete(ac.hardwareToValidator, hardwareID)

	return nil
}

// RegisterValidatorHardware registers the hardware binding for a validator
func (ac *SGXAdmissionController) RegisterValidatorHardware(validatorAddr common.Address, hardwareID string) error {
	ac.mu.Lock()
	defer ac.mu.Unlock()

	// Check if hardware already registered
	if existingValidator, exists := ac.hardwareToValidator[hardwareID]; exists {
		if existingValidator != validatorAddr {
			return ErrHardwareAlreadyRegistered
		}
	}

	ac.validatorToHardware[validatorAddr] = hardwareID
	ac.hardwareToValidator[hardwareID] = validatorAddr

	return nil
}

// SGXVerifierAdapter adapts internal/sgx.Verifier to governance.SGXVerifier interface
type SGXVerifierAdapter struct {
	verifier *sgx.DCAPVerifier
}

// NewSGXVerifierAdapter creates a new SGX verifier adapter
func NewSGXVerifierAdapter(allowOutdatedTCB bool) *SGXVerifierAdapter {
	return &SGXVerifierAdapter{
		verifier: sgx.NewDCAPVerifier(allowOutdatedTCB),
	}
}

// VerifyQuote verifies an SGX quote
func (a *SGXVerifierAdapter) VerifyQuote(quote []byte) error {
	return a.verifier.VerifyQuote(quote)
}

// ExtractMREnclave extracts the MRENCLAVE from a quote
func (a *SGXVerifierAdapter) ExtractMREnclave(quote []byte) ([32]byte, error) {
	parsedQuote, err := sgx.ParseQuote(quote)
	if err != nil {
		return [32]byte{}, err
	}
	return parsedQuote.MRENCLAVE, nil
}

// ExtractHardwareID extracts the hardware ID from a quote
func (a *SGXVerifierAdapter) ExtractHardwareID(quote []byte) (string, error) {
	instanceID, err := sgx.ExtractInstanceID(quote)
	if err != nil {
		return "", err
	}
	return instanceID.String(), nil
}
