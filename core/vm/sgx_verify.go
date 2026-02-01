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

package vm

import (
	"crypto/ed25519"
	"errors"

	"github.com/ethereum/go-ethereum/crypto"
)

// SGXVerify 签名验证预编译合约 (0x8003)
type SGXVerify struct{}

// Name returns the name of the contract
func (c *SGXVerify) Name() string {
	return "SGXVerify"
}

// RequiredGas 计算所需 Gas
// 输入格式: hash (32 bytes) + signature (65 bytes) + publicKey (64 bytes)
func (c *SGXVerify) RequiredGas(input []byte) uint64 {
	return 5000
}

// Run 执行合约（不需要上下文，纯计算）
// 输入格式: hash (32 bytes) + signature (variable) + publicKey (variable)
// 输出格式: result (1 byte: 0x01 for valid, 0x00 for invalid)
func (c *SGXVerify) Run(input []byte) ([]byte, error) {
	// ECDSA 验证: hash (32) + sig (65) + pubkey (64) = 161 bytes
	// Ed25519 验证: hash (32) + sig (64) + pubkey (32) = 128 bytes
	
	if len(input) == 161 {
		// ECDSA 验证
		hash := input[:32]
		signature := input[32:97]
		pubKey := input[97:161]
		
		// 恢复公钥
		recoveredPubKey, err := crypto.SigToPub(hash, signature)
		if err != nil {
			return []byte{0x00}, nil
		}
		
		// 比较公钥
		recoveredPubKeyBytes := crypto.FromECDSAPub(recoveredPubKey)
		// 去除前缀 0x04
		if len(recoveredPubKeyBytes) > 0 && recoveredPubKeyBytes[0] == 0x04 {
			recoveredPubKeyBytes = recoveredPubKeyBytes[1:]
		}
		
		// 比较
		if len(recoveredPubKeyBytes) != len(pubKey) {
			return []byte{0x00}, nil
		}
		for i := 0; i < len(pubKey); i++ {
			if recoveredPubKeyBytes[i] != pubKey[i] {
				return []byte{0x00}, nil
			}
		}
		
		return []byte{0x01}, nil
		
	} else if len(input) == 128 {
		// Ed25519 验证
		hash := input[:32]
		signature := input[32:96]
		pubKey := input[96:128]
		
		if ed25519.Verify(ed25519.PublicKey(pubKey), hash, signature) {
			return []byte{0x01}, nil
		}
		return []byte{0x00}, nil
		
	} else {
		return nil, errors.New("invalid input length: expected 161 bytes (ECDSA) or 128 bytes (Ed25519)")
	}
}
