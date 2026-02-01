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

package genesis

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)

// CalculateContractAddress deterministically calculates a contract address
// based on the deployer address and nonce using CREATE opcode rules
func CalculateContractAddress(deployer common.Address, nonce uint64) common.Address {
	// CREATE address calculation: keccak256(rlp([deployer, nonce]))
	data, _ := rlp.EncodeToBytes([]interface{}{deployer, nonce})
	hash := crypto.Keccak256Hash(data)
	
	var addr common.Address
	copy(addr[:], hash[12:])
	return addr
}

// CalculateCreate2Address calculates the address for CREATE2
func CalculateCreate2Address(deployer common.Address, salt [32]byte, initCodeHash [32]byte) common.Address {
	// CREATE2 address calculation: keccak256(0xff ++ deployer ++ salt ++ keccak256(init_code))
	data := make([]byte, 1+20+32+32)
	data[0] = 0xff
	copy(data[1:21], deployer[:])
	copy(data[21:53], salt[:])
	copy(data[53:85], initCodeHash[:])
	
	hash := crypto.Keccak256Hash(data)
	
	var addr common.Address
	copy(addr[:], hash[12:])
	return addr
}

// PredictGovernanceAddress predicts the governance contract address
// The governance contract is assumed to be deployed at nonce 0
func PredictGovernanceAddress(deployer common.Address) common.Address {
	return CalculateContractAddress(deployer, 0)
}

// PredictSecurityConfigAddress predicts the security config contract address
// The security config contract is assumed to be deployed at nonce 1
func PredictSecurityConfigAddress(deployer common.Address) common.Address {
	return CalculateContractAddress(deployer, 1)
}
