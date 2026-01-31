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
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

// Contract ABI for SecurityConfigContract
// This is a simplified ABI definition - in production, this would be generated from Solidity
const securityConfigABI = `[
	{
		"name": "getAllowedMREnclaves",
		"type": "function",
		"stateMutability": "view",
		"inputs": [],
		"outputs": [{"type": "bytes32[]"}]
	},
	{
		"name": "getAllowedMRSigners",
		"type": "function",
		"stateMutability": "view",
		"inputs": [],
		"outputs": [{"type": "bytes32[]"}]
	},
	{
		"name": "getISVProdID",
		"type": "function",
		"stateMutability": "view",
		"inputs": [],
		"outputs": [{"type": "uint16"}]
	},
	{
		"name": "getISVSVN",
		"type": "function",
		"stateMutability": "view",
		"inputs": [],
		"outputs": [{"type": "uint16"}]
	},
	{
		"name": "getCertValidityPeriod",
		"type": "function",
		"stateMutability": "view",
		"inputs": [],
		"outputs": [{"type": "uint256", "name": "notBefore"}, {"type": "uint256", "name": "notAfter"}]
	},
	{
		"name": "getAdmissionPolicy",
		"type": "function",
		"stateMutability": "view",
		"inputs": [],
		"outputs": [{"type": "bool"}]
	}
]`

// Contract ABI for GovernanceContract
const governanceContractABI = `[
	{
		"name": "getKeyMigrationThreshold",
		"type": "function",
		"stateMutability": "view",
		"inputs": [],
		"outputs": [{"type": "uint256"}]
	}
]`

// securityConfigContractCaller handles calls to the SecurityConfigContract
type securityConfigContractCaller struct {
	client   *ethclient.Client
	address  common.Address
	abi      abi.ABI
	testMode bool // If true, use mock data instead of actual calls
}

// newSecurityConfigContractCaller creates a new contract caller
func newSecurityConfigContractCaller(client *ethclient.Client, address common.Address, testMode bool) (*securityConfigContractCaller, error) {
	parsedABI, err := abi.JSON(strings.NewReader(securityConfigABI))
	if err != nil {
		return nil, fmt.Errorf("failed to parse contract ABI: %w", err)
	}

	return &securityConfigContractCaller{
		client:   client,
		address:  address,
		abi:      parsedABI,
		testMode: testMode,
	}, nil
}

// getAllowedMREnclaves fetches the MRENCLAVE whitelist from the contract
func (c *securityConfigContractCaller) getAllowedMREnclaves(ctx context.Context) ([]string, error) {
	if c.testMode {
		// Return mock data for testing
		return []string{}, nil
	}

	// Pack the function call
	data, err := c.abi.Pack("getAllowedMREnclaves")
	if err != nil {
		return nil, fmt.Errorf("failed to pack function call: %w", err)
	}

	// Call the contract
	msg := ethereum.CallMsg{
		To:   &c.address,
		Data: data,
	}

	result, err := c.client.CallContract(ctx, msg, nil)
	if err != nil {
		if c.testMode {
			// In test mode, ignore errors and return empty list
			return []string{}, nil
		}
		return nil, fmt.Errorf("contract call failed: %w", err)
	}

	// Unpack the result
	var mrenclaves [][32]byte
	err = c.abi.UnpackIntoInterface(&mrenclaves, "getAllowedMREnclaves", result)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack result: %w", err)
	}

	// Convert to hex strings
	resultStr := make([]string, len(mrenclaves))
	for i, mr := range mrenclaves {
		resultStr[i] = fmt.Sprintf("%x", mr)
	}

	return resultStr, nil
}

// getAllowedMRSigners fetches the MRSIGNER whitelist from the contract
func (c *securityConfigContractCaller) getAllowedMRSigners(ctx context.Context) ([]string, error) {
	if c.testMode {
		// Return mock data for testing
		return []string{}, nil
	}

	data, err := c.abi.Pack("getAllowedMRSigners")
	if err != nil {
		return nil, fmt.Errorf("failed to pack function call: %w", err)
	}

	msg := ethereum.CallMsg{
		To:   &c.address,
		Data: data,
	}

	result, err := c.client.CallContract(ctx, msg, nil)
	if err != nil {
		if c.testMode {
			return []string{}, nil
		}
		return nil, fmt.Errorf("contract call failed: %w", err)
	}

	var mrsigners [][32]byte
	err = c.abi.UnpackIntoInterface(&mrsigners, "getAllowedMRSigners", result)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack result: %w", err)
	}

	resultStr := make([]string, len(mrsigners))
	for i, mr := range mrsigners {
		resultStr[i] = fmt.Sprintf("%x", mr)
	}

	return resultStr, nil
}

// getISVProdID fetches the ISV Product ID from the contract
func (c *securityConfigContractCaller) getISVProdID(ctx context.Context) (uint16, error) {
	if c.testMode {
		return 0, nil
	}

	data, err := c.abi.Pack("getISVProdID")
	if err != nil {
		return 0, fmt.Errorf("failed to pack function call: %w", err)
	}

	msg := ethereum.CallMsg{
		To:   &c.address,
		Data: data,
	}

	result, err := c.client.CallContract(ctx, msg, nil)
	if err != nil {
		if c.testMode {
			return 0, nil
		}
		return 0, fmt.Errorf("contract call failed: %w", err)
	}

	var prodID uint16
	err = c.abi.UnpackIntoInterface(&prodID, "getISVProdID", result)
	if err != nil {
		return 0, fmt.Errorf("failed to unpack result: %w", err)
	}

	return prodID, nil
}

// getISVSVN fetches the ISV Security Version Number from the contract
func (c *securityConfigContractCaller) getISVSVN(ctx context.Context) (uint16, error) {
	if c.testMode {
		return 1, nil
	}

	data, err := c.abi.Pack("getISVSVN")
	if err != nil {
		return 0, fmt.Errorf("failed to pack function call: %w", err)
	}

	msg := ethereum.CallMsg{
		To:   &c.address,
		Data: data,
	}

	result, err := c.client.CallContract(ctx, msg, nil)
	if err != nil {
		if c.testMode {
			return 1, nil
		}
		return 0, fmt.Errorf("contract call failed: %w", err)
	}

	var svn uint16
	err = c.abi.UnpackIntoInterface(&svn, "getISVSVN", result)
	if err != nil {
		return 0, fmt.Errorf("failed to unpack result: %w", err)
	}

	return svn, nil
}

// getCertValidityPeriod fetches the certificate validity period from the contract
func (c *securityConfigContractCaller) getCertValidityPeriod(ctx context.Context) (string, string, error) {
	if c.testMode {
		return "0", fmt.Sprintf("%d", time.Now().Add(365*24*time.Hour).Unix()), nil
	}

	data, err := c.abi.Pack("getCertValidityPeriod")
	if err != nil {
		return "", "", fmt.Errorf("failed to pack function call: %w", err)
	}

	msg := ethereum.CallMsg{
		To:   &c.address,
		Data: data,
	}

	result, err := c.client.CallContract(ctx, msg, nil)
	if err != nil {
		if c.testMode {
			return "0", fmt.Sprintf("%d", time.Now().Add(365*24*time.Hour).Unix()), nil
		}
		return "", "", fmt.Errorf("contract call failed: %w", err)
	}

	var notBefore, notAfter *big.Int
	err = c.abi.UnpackIntoInterface(&[]interface{}{&notBefore, &notAfter}, "getCertValidityPeriod", result)
	if err != nil {
		return "", "", fmt.Errorf("failed to unpack result: %w", err)
	}

	return notBefore.String(), notAfter.String(), nil
}

// getAdmissionPolicy fetches the admission policy from the contract
func (c *securityConfigContractCaller) getAdmissionPolicy(ctx context.Context) (bool, error) {
	if c.testMode {
		return false, nil
	}

	data, err := c.abi.Pack("getAdmissionPolicy")
	if err != nil {
		return false, fmt.Errorf("failed to pack function call: %w", err)
	}

	msg := ethereum.CallMsg{
		To:   &c.address,
		Data: data,
	}

	result, err := c.client.CallContract(ctx, msg, nil)
	if err != nil {
		if c.testMode {
			return false, nil
		}
		return false, fmt.Errorf("contract call failed: %w", err)
	}

	var strict bool
	err = c.abi.UnpackIntoInterface(&strict, "getAdmissionPolicy", result)
	if err != nil {
		return false, fmt.Errorf("failed to unpack result: %w", err)
	}

	return strict, nil
}

// governanceContractCaller handles calls to the GovernanceContract
type governanceContractCaller struct {
	client   *ethclient.Client
	address  common.Address
	abi      abi.ABI
	testMode bool
}

// newGovernanceContractCaller creates a new governance contract caller
func newGovernanceContractCaller(client *ethclient.Client, address common.Address, testMode bool) (*governanceContractCaller, error) {
	parsedABI, err := abi.JSON(strings.NewReader(governanceContractABI))
	if err != nil {
		return nil, fmt.Errorf("failed to parse contract ABI: %w", err)
	}

	return &governanceContractCaller{
		client:   client,
		address:  address,
		abi:      parsedABI,
		testMode: testMode,
	}, nil
}

// getKeyMigrationThreshold fetches the key migration threshold from the contract
func (c *governanceContractCaller) getKeyMigrationThreshold(ctx context.Context) (uint64, error) {
	if c.testMode {
		return 3, nil
	}

	data, err := c.abi.Pack("getKeyMigrationThreshold")
	if err != nil {
		return 0, fmt.Errorf("failed to pack function call: %w", err)
	}

	msg := ethereum.CallMsg{
		To:   &c.address,
		Data: data,
	}

	result, err := c.client.CallContract(ctx, msg, nil)
	if err != nil {
		if c.testMode {
			return 3, nil
		}
		return 0, fmt.Errorf("contract call failed: %w", err)
	}

	var threshold *big.Int
	err = c.abi.UnpackIntoInterface(&threshold, "getKeyMigrationThreshold", result)
	if err != nil {
		return 0, fmt.Errorf("failed to unpack result: %w", err)
	}

	return threshold.Uint64(), nil
}
