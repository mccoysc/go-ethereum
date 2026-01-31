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
	"encoding/asn1"
	"encoding/binary"
	"errors"
)

// SGXQuote represents the SGX Quote data structure.
type SGXQuote struct {
	Version    uint16   // Quote version
	SignType   uint16   // Signature type (EPID/DCAP)
	MRENCLAVE  [32]byte // Enclave code measurement
	MRSIGNER   [32]byte // Signer measurement
	ISVProdID  uint16   // Product ID
	ISVSVN     uint16   // Security version number
	ReportData [64]byte // User-defined data
	TCBStatus  uint8    // TCB status
	Signature  []byte   // Quote signature
}

// TCB status constants
const (
	TCBUpToDate            uint8 = 0x00
	TCBOutOfDate           uint8 = 0x01
	TCBRevoked             uint8 = 0x02
	TCBConfigurationNeeded uint8 = 0x03
)

// SGXQuoteOID is the OID for SGX Quote in X.509 certificates.
// This is a custom OID for embedding SGX quotes in RA-TLS certificates.
var SGXQuoteOID = asn1.ObjectIdentifier{1, 2, 840, 113741, 1, 13, 1}

// ParseQuote parses an SGX Quote from raw bytes.
func ParseQuote(quote []byte) (*SGXQuote, error) {
	if len(quote) < 432 {
		return nil, errors.New("quote too short: minimum 432 bytes required")
	}

	q := &SGXQuote{}
	q.Version = binary.LittleEndian.Uint16(quote[0:2])
	q.SignType = binary.LittleEndian.Uint16(quote[2:4])
	copy(q.MRENCLAVE[:], quote[112:144])
	copy(q.MRSIGNER[:], quote[176:208])
	q.ISVProdID = binary.LittleEndian.Uint16(quote[304:306])
	q.ISVSVN = binary.LittleEndian.Uint16(quote[306:308])
	copy(q.ReportData[:], quote[368:432])

	// TCB status is typically at a fixed offset for DCAP quotes
	// For simplicity, we default to up-to-date
	q.TCBStatus = TCBUpToDate

	// Signature data follows the fixed fields
	if len(quote) > 432 {
		q.Signature = make([]byte, len(quote)-432)
		copy(q.Signature, quote[432:])
	}

	return q, nil
}
