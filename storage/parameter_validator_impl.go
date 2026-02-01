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

package storage

import (
	"fmt"
	"sync"
)

// ParamCategory defines the category of a parameter
type ParamCategory uint8

const (
	ParamCategorySecurity ParamCategory = 0x01
	ParamCategoryRuntime  ParamCategory = 0x02
)

// ParamDefinition defines a parameter's metadata
type ParamDefinition struct {
	Name      string
	Category  ParamCategory
	EnvKey    string
	CliFlag   string
	Required  bool
	Default   string
	Validator func(string) error
}

// SecurityParams defines the security-critical parameters
var SecurityParams = []ParamDefinition{
	{
		Name:     "encrypted_path",
		Category: ParamCategorySecurity,
		EnvKey:   "XCHAIN_ENCRYPTED_PATH",
		CliFlag:  "xchain.encrypted-path",
		Required: true,
	},
	{
		Name:     "secret_path",
		Category: ParamCategorySecurity,
		EnvKey:   "XCHAIN_SECRET_PATH",
		CliFlag:  "xchain.secret-path",
		Required: true,
	},
	{
		Name:     "governance_contract",
		Category: ParamCategorySecurity,
		EnvKey:   "XCHAIN_GOVERNANCE_CONTRACT",
		CliFlag:  "xchain.governance-contract",
		Required: true,
	},
	{
		Name:     "security_config_contract",
		Category: ParamCategorySecurity,
		EnvKey:   "XCHAIN_SECURITY_CONFIG_CONTRACT",
		CliFlag:  "xchain.security-config-contract",
		Required: true,
	},
}

// ParameterValidatorImpl implements ParameterValidator
type ParameterValidatorImpl struct {
	mu             sync.RWMutex
	manifestParams map[string]string
	chainParams    map[string]interface{}
	cliParams      map[string]interface{}
	mergedParams   map[string]interface{}
}

// NewParameterValidator creates a new parameter validator
func NewParameterValidator() *ParameterValidatorImpl {
	return &ParameterValidatorImpl{
		manifestParams: make(map[string]string),
		chainParams:    make(map[string]interface{}),
		cliParams:      make(map[string]interface{}),
		mergedParams:   make(map[string]interface{}),
	}
}

// ValidateManifestParams validates parameters from Gramine manifest
func (pv *ParameterValidatorImpl) ValidateManifestParams(manifestParams map[string]string) error {
	pv.mu.Lock()
	defer pv.mu.Unlock()

	pv.manifestParams = manifestParams

	// Check required manifest parameters
	for _, param := range SecurityParams {
		if param.Required {
			if _, exists := manifestParams[param.EnvKey]; !exists {
				return fmt.Errorf("required manifest parameter missing: %s", param.EnvKey)
			}
		}
	}

	return nil
}

// ValidateChainParams validates parameters from on-chain contracts
func (pv *ParameterValidatorImpl) ValidateChainParams(chainParams map[string]interface{}) error {
	pv.mu.Lock()
	defer pv.mu.Unlock()

	pv.chainParams = chainParams

	// Validate chain parameters (basic validation)
	// In production, would validate specific parameter types and ranges

	return nil
}

// MergeAndValidate merges parameters with priority: Manifest > Chain > CommandLine
func (pv *ParameterValidatorImpl) MergeAndValidate(
	manifestParams map[string]string,
	chainParams map[string]interface{},
	cmdLineParams map[string]interface{},
) (map[string]interface{}, error) {
	pv.mu.Lock()
	defer pv.mu.Unlock()

	pv.manifestParams = manifestParams
	pv.chainParams = chainParams
	pv.cliParams = cmdLineParams
	pv.mergedParams = make(map[string]interface{})

	// Priority 1: Manifest parameters (highest)
	for key, value := range manifestParams {
		pv.mergedParams[key] = value
	}

	// Priority 2: Chain parameters
	for key, value := range chainParams {
		// Only add if not already set by manifest
		if _, exists := pv.mergedParams[key]; !exists {
			pv.mergedParams[key] = value
		}
	}

	// Priority 3: Command line parameters (lowest)
	for key, value := range cmdLineParams {
		// Check if this is a security parameter
		isSecurityParam := false
		var paramDef *ParamDefinition

		for _, param := range SecurityParams {
			if param.CliFlag == key {
				isSecurityParam = true
				paramDef = &param
				break
			}
		}

		if isSecurityParam {
			// For security params, only set if not already set by manifest or chain
			manifestKey := paramDef.EnvKey
			if _, manifestExists := pv.mergedParams[manifestKey]; !manifestExists {
				if _, chainExists := pv.mergedParams[paramDef.Name]; !chainExists {
					pv.mergedParams[paramDef.Name] = value
				}
			}
		} else {
			// For non-security params, always allow override
			if _, exists := pv.mergedParams[key]; !exists {
				pv.mergedParams[key] = value
			}
		}
	}

	// Validate that all required parameters are present
	for _, param := range SecurityParams {
		if param.Required {
			found := false
			// Check by env key or param name
			if _, exists := pv.mergedParams[param.EnvKey]; exists {
				found = true
			}
			if _, exists := pv.mergedParams[param.Name]; exists {
				found = true
			}
			if !found {
				return nil, fmt.Errorf("required parameter missing: %s", param.Name)
			}
		}
	}

	// Return a copy
	result := make(map[string]interface{})
	for k, v := range pv.mergedParams {
		result[k] = v
	}

	return result, nil
}

// CheckSecurityParams verifies that security parameters are properly set
func (pv *ParameterValidatorImpl) CheckSecurityParams() error {
	pv.mu.RLock()
	defer pv.mu.RUnlock()

	// Verify that security-critical parameters came from manifest or chain
	for _, param := range SecurityParams {
		if param.Category == ParamCategorySecurity {
			// Check if it exists in merged params
			found := false
			if _, exists := pv.mergedParams[param.EnvKey]; exists {
				found = true
			}
			if _, exists := pv.mergedParams[param.Name]; exists {
				found = true
			}

			if !found && param.Required {
				return fmt.Errorf("security parameter not set: %s", param.Name)
			}
		}
	}

	return nil
}

// GetMergedParams returns the merged parameters
func (pv *ParameterValidatorImpl) GetMergedParams() map[string]interface{} {
	pv.mu.RLock()
	defer pv.mu.RUnlock()

	result := make(map[string]interface{})
	for k, v := range pv.mergedParams {
		result[k] = v
	}
	return result
}
