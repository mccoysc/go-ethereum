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

// ParameterValidator validates and merges parameters from different sources
type ParameterValidator interface {
	// ValidateManifestParams validates parameters from Gramine manifest (environment variables)
	ValidateManifestParams(manifestParams map[string]string) error

	// ValidateChainParams validates parameters from on-chain SecurityConfigContract
	ValidateChainParams(chainParams map[string]interface{}) error

	// MergeAndValidate merges parameters with priority: Manifest > Chain > CommandLine
	MergeAndValidate(
		manifestParams map[string]string,
		chainParams map[string]interface{},
		cmdLineParams map[string]interface{},
	) (map[string]interface{}, error)

	// CheckSecurityParams verifies that security parameters are properly set
	CheckSecurityParams() error
}
