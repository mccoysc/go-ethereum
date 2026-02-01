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
)

// BootstrapConfig holds the network bootstrap configuration
type BootstrapConfig struct {
	// AllowedMREnclave is the initial MRENCLAVE for the first version
	AllowedMREnclave [32]byte

	// MaxFounders is the maximum number of genesis founders
	MaxFounders uint64

	// VotingThreshold is the voting threshold percentage (e.g., 67 for 2/3)
	VotingThreshold uint64

	// GovernanceContract is the pre-deployed governance contract address
	GovernanceContract common.Address

	// SecurityConfigContract is the pre-deployed security config contract address
	SecurityConfigContract common.Address
}

// DefaultBootstrapConfig returns the default bootstrap configuration
func DefaultBootstrapConfig() *BootstrapConfig {
	return &BootstrapConfig{
		MaxFounders:     5,  // 最多 5 个创始管理者
		VotingThreshold: 67, // 2/3 投票通过
	}
}
