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
	"crypto/subtle"
)

// ConstantTimeCompare performs a constant-time comparison of two byte slices.
// The execution time is independent of whether the inputs are equal,
// providing protection against timing side-channel attacks.
func ConstantTimeCompare(a, b []byte) bool {
	return subtle.ConstantTimeCompare(a, b) == 1
}

// ConstantTimeCopy performs a constant-time conditional copy.
// If condition is true, src is copied to dst.
// The execution time is independent of the condition value.
func ConstantTimeCopy(condition bool, dst, src []byte) {
	mask := byte(0)
	if condition {
		mask = 0xFF
	}

	for i := range dst {
		if i < len(src) {
			dst[i] = (dst[i] & ^mask) | (src[i] & mask)
		}
	}
}

// ConstantTimeSelect performs a constant-time selection between two byte slices.
// If condition is true, returns a copy of a, otherwise returns a copy of b.
// The execution time is independent of the condition value.
func ConstantTimeSelect(condition bool, a, b []byte) []byte {
	if len(a) != len(b) {
		// For safety, return nil if lengths don't match
		return nil
	}

	result := make([]byte, len(a))
	mask := byte(0)
	if condition {
		mask = 0xFF
	}

	for i := range result {
		result[i] = (a[i] & mask) | (b[i] & ^mask)
	}
	return result
}
